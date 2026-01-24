package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/tui/components/command"
)

type closingOverlay struct {
	closed bool
}

func (o *closingOverlay) Init() tea.Cmd { return nil }

func (o *closingOverlay) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	o.closed = true
	return nil, nil
}

func (o *closingOverlay) View() (string, *tea.Cursor) {
	return lipgloss.NewStyle().Render("closing"), nil
}

func (o *closingOverlay) SetSize(width, height int) {}

func TestOverlayStackUpdateClosesOverlay(t *testing.T) {
	stack := newOverlayStack(20, 5)
	overlay := &closingOverlay{}

	if cmd := stack.Open(overlayKindHelp, overlay, command.OverlayPlacement{Fullscreen: true}); cmd != nil {
		_ = cmd()
	}
	if stack.ActiveKind() != overlayKindHelp {
		t.Fatalf("expected active kind help, got %v", stack.ActiveKind())
	}
	if !stack.HasOverlay() {
		t.Fatalf("expected overlay to be active")
	}

	closed, _ := stack.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if closed != overlayKindHelp {
		t.Fatalf("expected closed kind help, got %v", closed)
	}
	if stack.ActiveKind() != overlayKindNone {
		t.Fatalf("expected overlay stack to be cleared")
	}
	if stack.HasOverlay() {
		t.Fatalf("expected overlay pane to be empty")
	}
	if !overlay.closed {
		t.Fatalf("expected overlay update to run")
	}
}
