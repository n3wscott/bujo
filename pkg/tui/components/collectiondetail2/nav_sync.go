package collectiondetail2

import "tableflip.dev/bujo/pkg/tui/events"

// handleNavHighlight syncs nav highlighting into the detail pane selection.
func (m *Model) handleNavHighlight(msg events.CollectionHighlightMsg) {
	if m.sourceNav != "" && m.sourceNav != msg.Component {
		return
	}
	if msg.RowKind == "day" {
		m.ensurePlaceholderSection(msg.Collection)
	}
	if m.focused {
		m.focusSectionForCollection(msg.Collection)
		return
	}
	m.previewSectionForCollection(msg.Collection)
}

// handleNavSelect ensures missing day collections are materialized on selection.
func (m *Model) handleNavSelect(msg events.CollectionSelectMsg) {
	if m.sourceNav != "" && m.sourceNav != msg.Component {
		return
	}
	if !msg.Exists {
		m.ensurePlaceholderSection(msg.Collection)
		m.focusSectionForCollection(msg.Collection)
	}
}

func (m *Model) previewSectionForCollection(ref events.CollectionRef) bool {
	sectionIdx := m.sectionIndexForCollection(ref)
	if sectionIdx < 0 {
		return false
	}
	m.setActiveSection(sectionIdx)
	for idx, info := range m.lines {
		if info.section == sectionIdx && info.kind == lineHeader {
			m.scrollToLine(idx)
			break
		}
	}
	return true
}
