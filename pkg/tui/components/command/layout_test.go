package command

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea/v2"

	collectiondetail "tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	journalcomponent "tableflip.dev/bujo/pkg/tui/components/journal"
)

func TestCommandBarAnchorsWithScrolledDetail(t *testing.T) {
	width := 80
	commandHeight := 24
	contentHeight := commandHeight - 1

	cmd := NewModel(Options{
		ID:           "test-command",
		PromptPrefix: ":",
		StatusText:   "Ready",
	})
	cmd.SetSuggestions([]SuggestionOption{
		{Name: "today", Description: "Jump to today's collection"},
		{Name: "future", Description: "Future log"},
		{Name: "help", Description: "Show help"},
	})
	cmd.SetSize(width, commandHeight)

	const totalBullets = 60
	bullets := make([]collectiondetail.Bullet, 0, totalBullets)
	for i := 0; i < totalBullets; i++ {
		bullets = append(bullets, collectiondetail.Bullet{
			ID:    fmt.Sprintf("item-%02d", i),
			Label: fmt.Sprintf("Task %02d", i),
		})
	}

	sections := []collectiondetail.Section{{
		ID:       "Inbox",
		Title:    "Inbox",
		Bullets:  bullets,
		Subtitle: "",
	}}

	detail := collectiondetail.NewModel(sections)
	journal := journalcomponent.NewModel(nil, detail, nil)
	journal.SetSize(width, contentHeight)
	_ = journal.FocusDetail()

	for i := 0; i < 30; i++ {
		msg := tea.KeyPressMsg{Code: tea.KeyDown}
		_, _ = journal.Update(msg)
	}

	body, _ := journal.View()
	lines := strings.Split(body, "\n")
	if len(lines) != contentHeight {
		t.Fatalf("expected background to have %d lines, got %d", contentHeight, len(lines))
	}

	cmd.SetContent(body, nil)
	cmd.BeginInput("")

	view, _ := cmd.View()
	rendered := strings.Split(view, "\n")
	if !strings.Contains(view, "today") {
		t.Fatalf("expected suggestion overlay in view, got:\n%s", view)
	}
	if len(rendered) == 0 {
		t.Fatalf("expected rendered view to have lines")
	}

	last := rendered[len(rendered)-1]
	if !strings.Contains(last, ":") {
		t.Fatalf("expected command prompt on last line, got %q", last)
	}

	first := strings.TrimSpace(rendered[0])
	if strings.HasPrefix(first, ":") {
		t.Fatalf("expected content to precede prompt, got %q", rendered[0])
	}
}
