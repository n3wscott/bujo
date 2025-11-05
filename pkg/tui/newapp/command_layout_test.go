package newapp

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea/v2"

	collectiondetail "tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	journalcomponent "tableflip.dev/bujo/pkg/tui/components/journal"
)

func TestViewKeepsCommandBarAnchoredAfterDetailScroll(t *testing.T) {
	m := New(nil)
	m.width = 80
	m.height = 24

	const totalBullets = 80
	bullets := make([]collectiondetail.Bullet, 0, totalBullets)
	for i := 0; i < totalBullets; i++ {
		bullets = append(bullets, collectiondetail.Bullet{
			ID:    fmt.Sprintf("item-%02d", i),
			Label: fmt.Sprintf("Task %02d", i),
		})
	}
	sections := []collectiondetail.Section{{
		ID:      "Inbox",
		Title:   "Inbox",
		Bullets: bullets,
	}}

	detail := collectiondetail.NewModel(sections)
	journal := journalcomponent.NewModel(nil, detail, nil)
	journal.SetSize(m.width, m.height-1)
	if cmd := journal.FocusDetail(); cmd != nil {
		// ignore focus command in tests
	}
	for i := 0; i < 40; i++ {
		journal.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}

	m.journalView = journal
	m.journalDetail = detail
	m.layoutContent()

	m.command.BeginInput("")
	m.layoutContent()

	view, _ := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) == 0 {
		t.Fatalf("view unexpectedly empty")
	}
	if !strings.Contains(view, "today") {
		t.Fatalf("expected suggestion overlay in view, got:\n%s", view)
	}
	last := lines[len(lines)-1]
	if !strings.Contains(last, ":") {
		t.Fatalf("expected prompt on last line, got %q", last)
	}
	first := strings.TrimSpace(lines[0])
	if strings.HasPrefix(first, ":") {
		t.Fatalf("expected journal content above prompt, got %q", lines[0])
	}
}

func TestColonKeyKeepsCommandBarAnchoredAfterDetailScroll(t *testing.T) {
	m := New(nil)
	m.width = 90
	m.height = 28

	const totalBullets = 100
	bullets := make([]collectiondetail.Bullet, 0, totalBullets)
	for i := 0; i < totalBullets; i++ {
		bullets = append(bullets, collectiondetail.Bullet{
			ID:    fmt.Sprintf("item-%02d", i),
			Label: fmt.Sprintf("Task %02d", i),
		})
	}
	sections := []collectiondetail.Section{{
		ID:      "Inbox",
		Title:   "Inbox",
		Bullets: bullets,
	}}

	detail := collectiondetail.NewModel(sections)
	journal := journalcomponent.NewModel(nil, detail, nil)
	journal.SetSize(m.width, m.height-1)
	if cmd := journal.FocusDetail(); cmd != nil {
		m = drainCommands(t, m, cmd)
	}

	m.journalDetail = detail
	m.journalView = journal
	m.layoutContent()

	for i := 0; i < 60; i++ {
		next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		m = assertNewAppModel(t, next)
		m = drainCommands(t, m, cmd)
	}

	colon := tea.KeyPressMsg{Text: ":", Code: ':'}
	next, cmd := m.Update(colon)
	m = assertNewAppModel(t, next)
	m = drainCommands(t, m, cmd)

	view, _ := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) == 0 {
		t.Fatalf("view unexpectedly empty after entering command mode")
	}
	last := lines[len(lines)-1]
	if !strings.Contains(last, ":") {
		t.Fatalf("expected prompt on last line, got %q", last)
	}
	first := strings.TrimSpace(lines[0])
	if strings.HasPrefix(first, ":") {
		t.Fatalf("expected journal content above prompt, got %q", lines[0])
	}
}

func drainCommands(t *testing.T, m *Model, cmds ...tea.Cmd) *Model {
	queue := append([]tea.Cmd(nil), cmds...)
	for len(queue) > 0 {
		cmd := queue[0]
		queue = queue[1:]
		if cmd == nil {
			continue
		}
		msg := cmd()
		switch v := msg.(type) {
		case tea.BatchMsg:
			queue = append(queue, []tea.Cmd(v)...)
		default:
			next, nextCmd := m.Update(v)
			m = assertNewAppModel(t, next)
			if nextCmd != nil {
				queue = append(queue, nextCmd)
			}
		}
	}
	return m
}

func assertNewAppModel(t *testing.T, model tea.Model) *Model {
	t.Helper()
	m, ok := model.(*Model)
	if !ok {
		t.Fatalf("unexpected model type %T", model)
	}
	return m
}
