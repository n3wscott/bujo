package app

import (
	tea "github.com/charmbracelet/bubbletea/v2"
)

// activeOverlayKind returns the highest-priority visible overlay kind.
func (m *Model) activeOverlayKind() overlayKind {
	if m.overlayStack != nil {
		if kind := m.overlayStack.ActiveKind(); kind != overlayKindNone {
			return kind
		}
	}
	switch {
	case m.helpVisible:
		return overlayKindHelp
	case m.reportVisible:
		return overlayKindReport
	case m.addVisible:
		return overlayKindAdd
	case m.detailVisible:
		return overlayKindBulletDetail
	case m.moveVisible:
		return overlayKindMove
	case m.newCollectionVisible:
		return overlayKindNewCollection
	case m.migrateVisible:
		return overlayKindMigrate
	default:
		return overlayKindNone
	}
}

// overlayBlocksJournal reports whether overlays should block journal input.
func (m *Model) overlayBlocksJournal() bool {
	return m.activeOverlayKind() != overlayKindNone
}

// overlayBlocksCommand reports whether overlays should pause command updates.
func (m *Model) overlayBlocksCommand() bool {
	switch m.activeOverlayKind() {
	case overlayKindAdd, overlayKindBulletDetail, overlayKindMove, overlayKindNewCollection, overlayKindMigrate:
		return true
	default:
		return false
	}
}

// closeOverlay closes the requested overlay kind if visible.
func (m *Model) closeOverlay(kind overlayKind) tea.Cmd {
	switch kind {
	case overlayKindHelp:
		return m.closeHelpOverlay()
	case overlayKindReport:
		return m.closeReportOverlay()
	case overlayKindAdd:
		return m.closeAddTaskOverlay()
	case overlayKindBulletDetail:
		return m.closeBulletDetailOverlay()
	case overlayKindMove:
		return m.closeMoveOverlay()
	case overlayKindNewCollection:
		return m.closeNewCollectionOverlay()
	case overlayKindMigrate:
		return m.closeMigrateOverlay()
	default:
		return nil
	}
}

// dismissActiveOverlay closes whichever overlay is currently visible.
func (m *Model) dismissActiveOverlay() tea.Cmd {
	return m.closeOverlay(m.activeOverlayKind())
}
