package command

import (
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/tui/events"
)

// Focus gives the command prompt keyboard focus.
func (m *Model) Focus() {
	m.focused = true
	m.prompt.Focus()
}

// Blur releases focus.
func (m *Model) Blur() {
	m.focused = false
	m.prompt.Blur()
}

// BeginInput switches the command bar into input mode.
func (m *Model) BeginInput(initial string) tea.Cmd {
	m.mode = ModeInput
	m.prompt.SetValue(initial)
	m.lastPromptValue = initial
	m.prompt.CursorEnd()
	m.Focus()
	m.applySuggestionFilter(initial, true)
	return tea.Batch(m.prompt.Focus(), events.CommandChangeCmd(m.id, initial, events.CommandModeInput))
}

// ExitInput returns the command bar to passive mode.
func (m *Model) ExitInput() tea.Cmd {
	m.mode = ModePassive
	m.prompt.Blur()
	m.lastPromptValue = ""
	m.filteredSuggestions = nil
	m.suggestionOverlay = ""
	m.suggestionIndex = -1
	m.suggestionOriginal = ""
	m.suggestionWindowStart = 0
	return tea.Batch(events.CommandChangeCmd(m.id, "", events.CommandModePassive))
}

// Update routes messages to the command prompt and suggestion overlay.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	handledKey := false

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if handledKey {
			break
		}
		switch {
		case key.Matches(msg, m.keys.CancelInput):
			if m.mode == ModeInput {
				if m.clearSuggestionSelection() {
					handledKey = true
					newVal := m.prompt.Value()
					if newVal != m.lastPromptValue {
						m.lastPromptValue = newVal
						cmds = append(cmds, events.CommandChangeCmd(m.id, newVal, events.CommandModeInput))
					}
					break
				}
				handledKey = true
				cmds = append(cmds, m.ExitInput(), events.CommandCancelCmd(m.id))
				m.prompt.Blur()
			}
		case key.Matches(msg, m.keys.Submit):
			if m.mode == ModeInput {
				handledKey = true
				value := strings.TrimSpace(m.prompt.Value())
				if value != "" {
					cmds = append(cmds, events.CommandSubmitCmd(m.id, value))
					m.SetStatus(value)
				}
				cmds = append(cmds, m.ExitInput())
			}
		case key.Matches(msg, m.keys.SuggestPrev):
			if m.mode == ModeInput && m.cycleSuggestion(-1) {
				handledKey = true
				newVal := m.prompt.Value()
				if newVal != m.lastPromptValue {
					m.lastPromptValue = newVal
					cmds = append(cmds, events.CommandChangeCmd(m.id, newVal, events.CommandModeInput))
				}
			}
		case key.Matches(msg, m.keys.SuggestNext):
			if m.mode == ModeInput && m.cycleSuggestion(1) {
				handledKey = true
				newVal := m.prompt.Value()
				if newVal != m.lastPromptValue {
					m.lastPromptValue = newVal
					cmds = append(cmds, events.CommandChangeCmd(m.id, newVal, events.CommandModeInput))
				}
			}
		case m.mode == ModePassive && key.Matches(msg, m.keys.StartInput):
			cmds = append(cmds, m.BeginInput(""))
			return m, tea.Batch(cmds...)
		}
	}

	if !handledKey && m.mode == ModeInput {
		prev := m.prompt.Value()
		var cmd tea.Cmd
		m.prompt, cmd = m.prompt.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if newVal := m.prompt.Value(); newVal != prev {
			m.lastPromptValue = newVal
			m.applySuggestionFilter(newVal, true)
			cmds = append(cmds, events.CommandChangeCmd(m.id, newVal, events.CommandModeInput))
		}
	}

	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}
