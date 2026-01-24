package app

import journalcomponent "tableflip.dev/bujo/pkg/tui/components/journal"

// journal returns the active journal page model when available.
func (m *Model) journal() *journalcomponent.Model {
	if m.router == nil {
		return nil
	}
	return m.router.Journal()
}

// setJournal installs the journal page model on the router.
func (m *Model) setJournal(journal *journalcomponent.Model) {
	if m.router == nil {
		m.router = newPageRouter()
	}
	m.router.SetJournal(journal)
}
