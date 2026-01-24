package app

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
)

// setStatus updates the command bar and schedules an auto-clear for transient messages.
func (m *Model) setStatus(status string) {
	if m.command == nil {
		return
	}
	m.command.SetStatus(status)
	m.statusText = status
	m.statusSetEpoch = m.statusEpoch
	m.statusClearActive = false
	trim := strings.TrimSpace(status)
	if trim == "" || strings.EqualFold(trim, "ready") {
		m.statusClearPending = false
		return
	}
	m.statusClearPending = true
	m.statusClearToken++
}

// setStatusIfIdle updates the status only if nothing else has updated it in the current cycle.
func (m *Model) setStatusIfIdle(status string) {
	if m.statusSetEpoch == m.statusEpoch {
		return
	}
	m.setStatus(status)
}

func (m *Model) scheduleStatusClear() tea.Cmd {
	if !m.statusClearPending || m.statusClearActive || strings.TrimSpace(m.statusText) == "" {
		return nil
	}
	m.statusClearPending = false
	m.statusClearActive = true
	token := m.statusClearToken
	return tea.Tick(statusClearTimeout, func(time.Time) tea.Msg {
		return statusClearMsg{token: token}
	})
}

func (m *Model) clearStatus() {
	m.statusClearActive = false
	m.statusClearPending = false
	m.statusText = "Ready"
	if m.command != nil {
		m.command.SetStatus("Ready")
	}
}

// postInteractionStatus schedules a deferred status clear after user input.
func (m *Model) postInteractionStatus(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg:
		return m.scheduleStatusClear()
	}
	return nil
}
