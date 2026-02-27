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

func TestParseLabelAddArg(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "valid", raw: "Owner:Codex", want: "owner:codex"},
		{name: "missing value", raw: "owner:", wantErr: true},
		{name: "missing key", raw: ":codex", wantErr: true},
		{name: "missing separator", raw: "owner", wantErr: true},
		{name: "invalid characters", raw: "owner:co dex", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLabelAddArg(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestParseLabelRemoveArg(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "valid", raw: "Owner", want: "owner"},
		{name: "empty", raw: "   ", wantErr: true},
		{name: "key value", raw: "owner:codex", wantErr: true},
		{name: "invalid characters", raw: "owner team", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLabelRemoveArg(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestLabelsWithKey(t *testing.T) {
	labels := []string{"area:api", "owner:codex", "owner:snichols", "state:open"}
	got := labelsWithKey(labels, "owner")
	if len(got) != 2 {
		t.Fatalf("expected 2 labels for owner, got %d", len(got))
	}
	if got[0] != "owner:codex" || got[1] != "owner:snichols" {
		t.Fatalf("unexpected labels for owner: %v", got)
	}
}

func TestHandleCommandSubmitLabelUsage(t *testing.T) {
	m := NewWithOptions(Options{})
	msg := events.CommandSubmitMsg{
		Component: m.command.ID(),
		Value:     "label",
	}
	if _, handled := m.handleCommandSubmit(msg); !handled {
		t.Fatal("expected submit to be handled")
	}
	if m.statusText != labelCommandUsageLine {
		t.Fatalf("expected label usage status, got %q", m.statusText)
	}
}

func TestHandleCommandSubmitLabelShorthandInfersAdd(t *testing.T) {
	m := NewWithOptions(Options{})
	msg := events.CommandSubmitMsg{
		Component: m.command.ID(),
		Value:     "label owner:codex",
	}
	if _, handled := m.handleCommandSubmit(msg); !handled {
		t.Fatal("expected submit to be handled")
	}
	if m.statusText != "Add label unavailable: journal cache offline" {
		t.Fatalf("expected shorthand to route to add, got %q", m.statusText)
	}
}
