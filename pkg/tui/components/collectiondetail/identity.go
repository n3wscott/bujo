package collectiondetail

import "tableflip.dev/bujo/pkg/tui/events"

// SetID overrides the emitted component identifier.
func (m *Model) SetID(id events.ComponentID) {
	if id == "" {
		m.id = events.ComponentID("collectiondetail")
		return
	}
	m.id = id
}

// ID returns the component identifier.
func (m *Model) ID() events.ComponentID {
	return m.id
}
