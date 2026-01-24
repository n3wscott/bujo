package app

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/tui/components/command"
	journalcomponent "tableflip.dev/bujo/pkg/tui/components/journal"
	"tableflip.dev/bujo/pkg/tui/events"
)

func (m *Model) startNewCollectionPrompt() tea.Cmd {
	m.ensureOverlayStack()
	if m.newCollectionVisible {
		return m.closeNewCollectionOverlay()
	}
	overlay := newNewCollectionOverlay(m.dump)
	overlay.SetSize(m.width, maxInt(1, m.height-1))

	var cmds []tea.Cmd
	if m.helpVisible {
		if cmd := m.closeHelpOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.reportVisible {
		if cmd := m.closeReportOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.addVisible {
		if cmd := m.closeAddTaskOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.detailVisible {
		if cmd := m.closeBulletDetailOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.moveVisible {
		if cmd := m.closeMoveOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.migrateVisible {
		if cmd := m.closeMigrateOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	placement := command.OverlayPlacement{Fullscreen: true}
	if cmd := m.overlayStack.Open(overlayKindNewCollection, overlay, placement); cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.newCollectionOverlay = overlay
	m.newCollectionVisible = true
	_ = m.dropFocusKind(focusKindCommand)
	m.pushFocus(focusTarget{kind: focusKindOverlay, overlay: overlayKindNewCollection})
	cmds = append(cmds, m.blurJournalPanes()...)
	if focus := m.overlayStack.Focus(); focus != nil {
		cmds = append(cmds, focus)
	}
	if m.command != nil {
		m.setStatus("Enter a name for the new collection")
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) handleNewCollectionCreate(name string) tea.Cmd {
	name = strings.TrimSpace(name)
	if name == "" {
		if m.command != nil {
			m.setStatus("Collection name cannot be empty")
		}
		return nil
	}
	if m.service == nil {
		if m.command != nil {
			m.setStatus("Create failed: service offline")
		}
		return m.closeNewCollectionOverlayWithStatus("Create failed: service offline")
	}
	ctx := context.Background()
	if err := m.service.EnsureCollectionOfType(ctx, name, collection.TypeGeneric); err != nil {
		if m.command != nil {
			m.setStatus("Create failed: " + err.Error())
		}
		return nil
	}

	var cmds []tea.Cmd
	if m.journalNav != nil {
		ref := events.CollectionRef{ID: name, Name: name, Type: collection.TypeGeneric}
		if cmd := m.journalNav.SelectCollection(ref); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if cache := m.journalCache; cache != nil {
		if cmd := m.collectionSyncCmd(name); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if snap := m.snapshotSyncCmd(); snap != nil {
		cmds = append(cmds, snap)
	}
	if cmd := m.closeNewCollectionOverlayWithStatus("Created " + name); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) closeNewCollectionOverlayWithStatus(status string) tea.Cmd {
	if !m.newCollectionVisible {
		if status != "" && m.command != nil {
			m.setStatus(status)
		}
		return nil
	}
	var cmds []tea.Cmd
	if m.overlayStack != nil {
		if cmd := m.overlayStack.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.overlayStack.Close(overlayKindNewCollection)
	}
	m.newCollectionOverlay = nil
	m.newCollectionVisible = false
	_, _ = m.popFocusKind(focusKindOverlay)
	if cmd := m.restoreFocusAfterOverlay(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if status != "" && m.command != nil {
		m.setStatus(status)
	}
	if cmd := m.journalFocusCmd(journalcomponent.FocusNav); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) closeNewCollectionOverlay() tea.Cmd {
	return m.closeNewCollectionOverlayWithStatus("")
}
