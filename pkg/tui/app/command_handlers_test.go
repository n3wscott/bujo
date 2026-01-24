package app

import (
	"testing"

	"tableflip.dev/bujo/pkg/tui/events"
)

func TestHandleCommandSubmitEmptyShowsUsage(t *testing.T) {
	m := NewWithOptions(Options{})
	msg := events.CommandSubmitMsg{
		Component: m.command.ID(),
		Value:     "   ",
	}

	if _, handled := m.handleCommandSubmit(msg); !handled {
		t.Fatal("expected submit to be handled")
	}
	if m.statusText != commandUsageLine {
		t.Fatalf("expected usage status, got %q", m.statusText)
	}
}

func TestHandleCommandChangeTogglesCommandActive(t *testing.T) {
	m := NewWithOptions(Options{})

	enter := events.CommandChangeMsg{
		Component: m.command.ID(),
		Mode:      events.CommandModeInput,
	}
	if _, handled := m.handleCommandChange(enter); !handled {
		t.Fatal("expected change to be handled")
	}
	if !m.commandActive {
		t.Fatal("expected command to be active after entering input mode")
	}

	exit := events.CommandChangeMsg{
		Component: m.command.ID(),
		Mode:      events.CommandModePassive,
	}
	if _, handled := m.handleCommandChange(exit); !handled {
		t.Fatal("expected change to be handled")
	}
	if m.commandActive {
		t.Fatal("expected command to be inactive after leaving input mode")
	}
}
