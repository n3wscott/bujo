package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/tui/components/addtask"
	"tableflip.dev/bujo/pkg/tui/components/command"
	"tableflip.dev/bujo/pkg/tui/events"
)

func (m *Model) handleAddTaskRequest(msg events.AddTaskRequestMsg) tea.Cmd {
	if msg.CollectionID == "" {
		if m.command != nil {
			m.setStatus("Add task unavailable: missing collection")
		}
		return nil
	}
	if m.journalCache == nil {
		if m.command != nil {
			m.setStatus("Add task unavailable: journal cache offline")
		}
		return nil
	}
	opts := addtask.Options{
		InitialCollectionID:    msg.CollectionID,
		InitialCollectionLabel: strings.TrimSpace(msg.CollectionLabel),
		InitialParentBulletID:  strings.TrimSpace(msg.ParentBulletID),
	}
	return m.openAddTaskOverlay(opts, msg)
}

func (m *Model) openAddTaskOverlay(opts addtask.Options, req events.AddTaskRequestMsg) tea.Cmd {
	m.ensureOverlayStack()
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
	model := addtask.NewModel(m.journalCache, opts)
	model.SetID(addTaskOverlayID)
	wrapper := newAddtaskOverlay(model)
	placement := command.OverlayPlacement{Fullscreen: true}
	if cmd := m.overlayStack.Open(overlayKindAdd, wrapper, placement); cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.addOverlay = wrapper
	m.addVisible = true
	_ = m.dropFocusKind(focusKindCommand)
	m.pushFocus(focusTarget{kind: focusKindOverlay, overlay: overlayKindAdd})
	cmds = append(cmds, m.blurJournalPanes()...)
	if focusCmd := m.overlayStack.Focus(); focusCmd != nil {
		cmds = append(cmds, focusCmd)
	}
	label := strings.TrimSpace(req.CollectionLabel)
	if label == "" {
		label = req.CollectionID
	}
	if m.command != nil {
		m.setStatus("Add task overlay opened for " + label)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) closeAddTaskOverlay() tea.Cmd {
	if !m.addVisible {
		return nil
	}
	var cmds []tea.Cmd
	if m.overlayStack != nil {
		if cmd := m.overlayStack.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.overlayStack.Close(overlayKindAdd)
	}
	m.addOverlay = nil
	m.addVisible = false
	_, _ = m.popFocusKind(focusKindOverlay)
	if cmd := m.restoreFocusAfterOverlay(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if m.command != nil {
		m.setStatusIfIdle("Add task overlay closed")
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}
