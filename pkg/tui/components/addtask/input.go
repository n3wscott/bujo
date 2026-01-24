package addtask

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/events"
)

const cacheParentMetaKey = "parent_id"

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	if m.confirmReset {
		switch {
		case key.Matches(msg, m.keys.ConfirmYes):
			m.resetForm()
			m.confirmReset = false
			return m.blurCmd()
		case key.Matches(msg, m.keys.ConfirmNo):
			m.confirmReset = false
		}
		return nil
	}

	if !m.focused {
		return nil
	}

	var cmds []tea.Cmd

	switch {
	case key.Matches(msg, m.keys.NextField):
		m.advanceFocus(1)
	case key.Matches(msg, m.keys.PrevField):
		m.advanceFocus(-1)
	case key.Matches(msg, m.keys.MoveUp):
		m.adjustSelection(-1)
	case key.Matches(msg, m.keys.MoveDown):
		m.adjustSelection(1)
	case key.Matches(msg, m.keys.MoveLeft):
		m.adjustSelection(-1)
	case key.Matches(msg, m.keys.MoveRight):
		m.adjustSelection(1)
	case key.Matches(msg, m.keys.Submit):
		if cmd, err := m.submit(); err != nil {
			m.errorMsg = err.Error()
		} else if cmd != nil {
			cmds = appendCmd(cmds, cmd)
		}
	case key.Matches(msg, m.keys.Cancel):
		m.confirmReset = true
	}
	cmds = appendCmd(cmds, m.updateInputFocus())
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func appendCmd(cmds []tea.Cmd, cmd tea.Cmd) []tea.Cmd {
	if cmd == nil {
		return cmds
	}
	return append(cmds, cmd)
}

func (m *Model) updateInputFocus() tea.Cmd {
	if !m.focused {
		m.taskInput.Blur()
		return nil
	}
	if m.focus == fieldTaskInput {
		return m.taskInput.Focus()
	}
	m.taskInput.Blur()
	return nil
}

func (m *Model) advanceFocus(delta int) {
	seq := m.focusSequence()
	if len(seq) == 0 {
		return
	}
	current := 0
	for i, f := range seq {
		if f == m.focus {
			current = i
			break
		}
	}
	current = (current + len(seq) + delta) % len(seq)
	m.focus = seq[current]
	m.updateInputFocus()
}

func (m *Model) focusSequence() []focusField {
	return []focusField{
		fieldParentBullet,
		fieldBulletType,
		fieldSignifier,
		fieldTaskInput,
	}
}

func (m *Model) adjustSelection(delta int) {
	switch m.focus {
	case fieldParentBullet:
		if len(m.parentOptions) == 0 {
			return
		}
		m.parentIndex = clampIndex(m.parentIndex+delta, len(m.parentOptions))
	case fieldBulletType:
		if len(m.bulletOptions) == 0 {
			return
		}
		m.bulletIndex = clampIndex(m.bulletIndex+delta, len(m.bulletOptions))
		m.updatePrompt()
	case fieldSignifier:
		if len(m.signifierOptions) == 0 {
			return
		}
		m.signifierIndex = clampIndex(m.signifierIndex+delta, len(m.signifierOptions))
		m.updatePrompt()
	}
}

func (m *Model) submit() (tea.Cmd, error) {
	label := strings.TrimSpace(m.taskInput.Value())
	if label == "" {
		return nil, fmt.Errorf("task description is required")
	}
	targetID := m.resolveTargetCollection()
	if targetID == "" {
		return nil, fmt.Errorf("select a collection first")
	}
	parentID := ""
	if m.parentIndex > 0 && m.parentIndex < len(m.parentOptions) {
		parentID = m.parentOptions[m.parentIndex].ID
	}
	bulletID := fmt.Sprintf("tmp-%d", time.Now().UnixNano())
	bullet := collectiondetail.Bullet{
		ID:        bulletID,
		Label:     label,
		Bullet:    m.bulletOptions[m.bulletIndex],
		Signifier: m.signifierOptions[m.signifierIndex],
		Created:   time.Now(),
	}

	metaMap := map[string]string{}
	if parentID != "" {
		metaMap[cacheParentMetaKey] = parentID
	}
	if len(metaMap) == 0 {
		metaMap = nil
	}

	if err := m.cache.CreateBulletWithMeta(targetID, bullet, metaMap); err != nil {
		return nil, err
	}
	m.resetForm()
	m.lastSubmitted = time.Now()
	return m.blurCmd(), nil
}

func (m *Model) resetForm() {
	m.taskInput.SetValue("")
	m.parentIndex = 0
	m.errorMsg = ""
	m.confirmReset = false
	m.refreshParentOptions()
}

func (m *Model) updatePrompt() {
	bullet := m.bulletOptions[m.bulletIndex]
	signifier := m.signifierOptions[m.signifierIndex]
	prompt := describePrompt(bullet, signifier, m.currentParentLabel())
	m.taskInput.Placeholder = prompt
}

func (m *Model) currentParentLabel() string {
	if m.parentIndex > 0 && m.parentIndex < len(m.parentOptions) {
		return m.parentOptions[m.parentIndex].Label
	}
	return ""
}

func (m *Model) blurCmd() tea.Cmd {
	if !m.focused {
		return nil
	}
	m.focused = false
	m.updateInputFocus()
	return events.BlurCmd(m.id)
}
