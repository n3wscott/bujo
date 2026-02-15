package addtask

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/glyph"
)

const (
	overlayFrameCursorOffsetX = 3 // rounded border + left padding
	overlayFrameCursorOffsetY = 2 // rounded border + top padding
)

// View renders the overlay UI.
func (m *Model) View() (string, *tea.Cursor) {
	lines := []string{m.sectionTitle("Add Task")}
	lines = append(lines, m.renderCollectionRow())
	lines = append(lines, m.renderParentRow(), "")

	controlRow, controlPrefix := m.renderControlRow()
	controlRowIndex := len(lines)
	lines = append(lines, controlRow, "", m.renderStatusLine())

	bodyContent := lipgloss.JoinVertical(lipgloss.Left, lines...)
	maxContent := m.width - 12
	if maxContent < 20 {
		maxContent = m.width - 8
	}
	if maxContent < 16 {
		maxContent = 16
	}
	contentWidth := clampInt(m.fieldWidth, 16, maxContent)
	body := lipgloss.NewStyle().Width(contentWidth).Render(bodyContent)

	var cursor *tea.Cursor
	if c := m.taskInput.Cursor(); c != nil {
		clone := *c
		clone.X += controlPrefix
		clone.Y += controlRowIndex
		clone.X += overlayFrameCursorOffsetX
		clone.Y += overlayFrameCursorOffsetY
		cursor = &clone
	}

	frameStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("212")).
		Padding(1, 2)
	if !m.focused {
		frameStyle = frameStyle.BorderForeground(lipgloss.Color("240"))
	}
	box := frameStyle.Render(body)

	return box, cursor
}

func (m *Model) renderStatusLine() string {
	switch {
	case m.confirmReset:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Render("Press 'y' to discard draft, 'n' to continue editing.")
	case m.errorMsg != "":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Render(m.errorMsg)
	default:
		return "Enter to submit • Esc to clear • Tab between fields"
	}
}

func (m *Model) sectionTitle(title string) string {
	return lipgloss.NewStyle().Bold(true).Render(title)
}

func (m *Model) renderCollectionRow() string {
	if len(m.collectionOptions) == 0 {
		return m.renderRow("Collection:", "(none)", false)
	}
	if m.collectionIndex >= len(m.collectionOptions) {
		m.collectionIndex = len(m.collectionOptions) - 1
	}
	if m.collectionIndex < 0 {
		m.collectionIndex = 0
	}
	value := m.collectionOptions[m.collectionIndex].Label
	return m.renderRow("Collection:", value, false)
}

func (m *Model) renderParentRow() string {
	value := "(none)"
	if len(m.parentOptions) > 0 && m.parentIndex < len(m.parentOptions) {
		value = m.parentOptions[m.parentIndex].Label
	}
	return m.renderRow("Parent bullet:", value, m.focus == fieldParentBullet)
}

func (m *Model) renderControlRow() (string, int) {
	signifier := m.signifierOptions[m.signifierIndex]
	signifierLabel := symbolForSignifier(signifier)
	signifierBox := m.renderBox(signifierLabel, m.focus == fieldSignifier)

	bullet := m.bulletOptions[m.bulletIndex]
	bulletLabel := symbolForBullet(bullet)
	bulletBox := m.renderBox(bulletLabel, m.focus == fieldBulletType)

	cursorGlyph := "➤"
	if m.focus == fieldTaskInput && m.focused {
		cursorGlyph = lipgloss.NewStyle().Foreground(focusColor).Render(cursorGlyph)
	}

	prefix := fmt.Sprintf("  %s %s %s ", signifierBox, bulletBox, cursorGlyph)
	line := prefix + m.taskInput.View()
	return line, lipgloss.Width(prefix)
}

func (m *Model) renderRow(label, value string, focused bool) string {
	indicator := "  "
	labelStyle := lipgloss.NewStyle().Bold(false)
	valueStyle := lipgloss.NewStyle()
	if focused {
		style := lipgloss.NewStyle().Foreground(focusColor)
		indicator = style.Render("➤ ")
		labelStyle = labelStyle.Foreground(focusColor)
		valueStyle = valueStyle.Foreground(focusColor)
	}
	return indicator + labelStyle.Render(fmt.Sprintf("%-13s", label)) + " " + valueStyle.Render(value)
}

func (m *Model) renderBox(content string, focused bool) string {
	if strings.TrimSpace(content) == "" {
		content = " "
	}
	box := fmt.Sprintf("[%s]", content)
	if focused {
		return lipgloss.NewStyle().Foreground(focusColor).Render(box)
	}
	return box
}

func symbolForBullet(b glyph.Bullet) string {
	if info, ok := glyph.DefaultBullets()[b]; ok {
		if s := strings.TrimSpace(info.Symbol); s != "" {
			return s
		}
	}
	return string(b)
}

func symbolForSignifier(s glyph.Signifier) string {
	if info, ok := glyph.DefaultSignifiers()[s]; ok {
		if sym := strings.TrimSpace(info.Symbol); sym != "" {
			return sym
		}
	}
	return " "
}
