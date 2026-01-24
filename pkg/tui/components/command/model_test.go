package command

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/tui/events"
)

func TestColonStartsInputMode(t *testing.T) {
	model := NewModel(Options{PromptPrefix: ":"})
	_, cmd := model.Update(tea.KeyPressMsg{Text: ":", Code: ':'})
	if model.mode != ModeInput {
		t.Fatalf("expected mode input after ':'")
	}
	if cmd == nil {
		t.Fatalf("expected command change cmd")
	}
	change := findCommandChange(runCmd(cmd))
	if change == nil {
		t.Fatalf("expected command change message")
	}
	if change.Mode != events.CommandModeInput {
		t.Fatalf("expected command mode input, got %q", change.Mode)
	}
}

func TestBeginAndExitInput(t *testing.T) {
	model := NewModel(Options{PromptPrefix: ":"})
	cmd := model.BeginInput(":today")
	if model.mode != ModeInput {
		t.Fatalf("expected mode input")
	}
	if model.prompt.Value() != ":today" {
		t.Fatalf("expected prompt value to be set")
	}
	change := findCommandChange(runCmd(cmd))
	if change == nil || change.Value != ":today" {
		t.Fatalf("expected change msg value :today, got %#v", change)
	}

	cmd = model.ExitInput()
	if model.mode != ModePassive {
		t.Fatalf("expected mode passive")
	}
	change = findCommandChange(runCmd(cmd))
	if change == nil || change.Mode != events.CommandModePassive {
		t.Fatalf("expected command mode passive, got %#v", change)
	}
}

func TestSuggestionCycleAndEscClearsSelection(t *testing.T) {
	model := NewModel(Options{PromptPrefix: ":"})
	model.SetSize(40, 5)
	model.BeginInput("")
	model.SetSuggestions([]SuggestionOption{{Name: ":today"}, {Name: ":future"}})
	model.applySuggestionFilter("", true)

	if len(model.filteredSuggestions) == 0 {
		t.Fatalf("expected filtered suggestions to be populated")
	}

	model.cycleSuggestion(1)
	if model.suggestionIndex < 0 {
		t.Fatalf("expected suggestion to be selected")
	}
	if model.prompt.Value() == "" {
		t.Fatalf("expected prompt to be filled by suggestion")
	}

	model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if model.suggestionIndex != -1 {
		t.Fatalf("expected suggestion selection to be cleared")
	}
	if model.mode != ModeInput {
		t.Fatalf("expected to remain in input mode after clearing suggestion")
	}
}

// runCmd executes a command and unwraps any batch messages.
func runCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	switch v := msg.(type) {
	case tea.BatchMsg:
		var msgs []tea.Msg
		for _, inner := range v {
			msgs = append(msgs, runCmd(inner)...)
		}
		return msgs
	default:
		return []tea.Msg{msg}
	}
}

func findCommandChange(msgs []tea.Msg) *events.CommandChangeMsg {
	for _, msg := range msgs {
		if change, ok := msg.(events.CommandChangeMsg); ok {
			return &change
		}
	}
	return nil
}
