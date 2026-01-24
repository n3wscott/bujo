package app

import (
	tea "github.com/charmbracelet/bubbletea/v2"

	journalcomponent "tableflip.dev/bujo/pkg/tui/components/journal"
	"tableflip.dev/bujo/pkg/tui/events"
)

// journalFocusCmd emits a focus request event for the active journal page.
func (m *Model) journalFocusCmd(pane journalcomponent.FocusPane) tea.Cmd {
	journal := m.journal()
	if journal == nil {
		return nil
	}
	target := events.JournalFocusNav
	if pane == journalcomponent.FocusDetail {
		target = events.JournalFocusDetail
	}
	id := journal.ID()
	return func() tea.Msg {
		return events.JournalFocusMsg{
			Component: id,
			Pane:      target,
		}
	}
}
