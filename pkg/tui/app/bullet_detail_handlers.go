package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/tui/components/bulletdetail"
	"tableflip.dev/bujo/pkg/tui/components/command"
	"tableflip.dev/bujo/pkg/tui/events"
)

// loadBulletDetail fetches an entry payload for the detail or move overlays.
func (m *Model) loadBulletDetail(collectionID, bulletID, requestID string) tea.Cmd {
	svc := m.service
	return func() tea.Msg {
		if svc == nil {
			return bulletDetailLoadedMsg{requestID: requestID, err: fmt.Errorf("service unavailable")}
		}
		ctx := context.Background()
		entries, err := svc.Entries(ctx, collectionID)
		if err != nil {
			return bulletDetailLoadedMsg{requestID: requestID, err: err}
		}
		for _, e := range entries {
			if e == nil {
				continue
			}
			if strings.TrimSpace(e.ID) == bulletID {
				e.EnsureHistorySeed()
				return bulletDetailLoadedMsg{requestID: requestID, entry: e}
			}
		}
		return bulletDetailLoadedMsg{requestID: requestID, err: fmt.Errorf("entry not found")}
	}
}

func (m *Model) handleBulletDetailRequest(msg events.BulletDetailRequestMsg) tea.Cmd {
	bulletID := strings.TrimSpace(msg.Bullet.ID)
	if bulletID == "" {
		if m.command != nil {
			m.setStatus("Bullet details unavailable: missing bullet ID")
		}
		return nil
	}
	if m.service == nil {
		if m.command != nil {
			m.setStatus("Bullet details unavailable: service offline")
		}
		return nil
	}
	collectionID := strings.TrimSpace(msg.Collection.ID)
	if collectionID == "" {
		collectionID = strings.TrimSpace(msg.Bullet.Note)
	}
	if collectionID == "" {
		if m.command != nil {
			m.setStatus("Bullet details unavailable: missing collection context")
		}
		return nil
	}
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
	if m.detailVisible {
		if cmd := m.closeBulletDetailOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.addVisible {
		if cmd := m.closeAddTaskOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	title := msg.Collection.Title
	if strings.TrimSpace(title) == "" {
		title = collectionID
	}
	detailModel := bulletdetail.New(title, msg.Bullet.Label, collectionID, msg.Bullet.Note)
	detailModel.SetLoading(true)
	wrapper := newBulletdetailOverlay(detailModel)
	placement := command.OverlayPlacement{Fullscreen: true}
	if cmd := m.overlayStack.Open(overlayKindBulletDetail, wrapper, placement); cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.detailOverlay = wrapper
	m.detailVisible = true
	requestID := fmt.Sprintf("%s@%d", bulletID, time.Now().UnixNano())
	m.detailLoadID = requestID
	_ = m.dropFocusKind(focusKindCommand)
	m.pushFocus(focusTarget{kind: focusKindOverlay, overlay: overlayKindBulletDetail})
	cmds = append(cmds, m.blurJournalPanes()...)
	if focusCmd := m.overlayStack.Focus(); focusCmd != nil {
		cmds = append(cmds, focusCmd)
	}
	if m.command != nil {
		statusLabel := strings.TrimSpace(msg.Bullet.Label)
		if statusLabel == "" {
			statusLabel = bulletID
		}
		m.setStatus("Loading details for " + statusLabel)
	}
	if loadCmd := m.loadBulletDetail(collectionID, bulletID, requestID); loadCmd != nil {
		cmds = append(cmds, loadCmd)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) closeBulletDetailOverlay() tea.Cmd {
	if !m.detailVisible {
		return nil
	}
	var cmds []tea.Cmd
	if m.overlayStack != nil {
		if cmd := m.overlayStack.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.overlayStack.Close(overlayKindBulletDetail)
	}
	m.detailOverlay = nil
	m.detailVisible = false
	m.detailLoadID = ""
	_, _ = m.popFocusKind(focusKindOverlay)
	if cmd := m.restoreFocusAfterOverlay(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if m.command != nil {
		m.setStatusIfIdle("Bullet detail overlay closed")
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) handleBulletDetailLoaded(msg bulletDetailLoadedMsg) tea.Cmd {
	if msg.requestID == "" || msg.requestID != m.detailLoadID {
		if msg.requestID != "" && msg.requestID == m.moveLoadID {
			return m.handleMoveDetailLoaded(msg)
		}
		return nil
	}
	if m.detailOverlay == nil {
		return nil
	}
	model := m.detailOverlay.Model()
	if model == nil {
		return nil
	}
	if msg.err != nil {
		model.SetError(msg.err)
		if m.command != nil {
			m.setStatus("Bullet detail error: " + msg.err.Error())
		}
		return nil
	}
	if msg.entry == nil {
		model.SetError(fmt.Errorf("entry not available"))
		return nil
	}
	model.SetEntry(msg.entry)
	if m.command != nil {
		label := strings.TrimSpace(msg.entry.Message)
		if label == "" {
			label = msg.entry.ID
		}
		m.setStatus("Loaded bullet details for " + label)
	}
	return nil
}

func (m *Model) handleMoveDetailLoaded(msg bulletDetailLoadedMsg) tea.Cmd {
	if m.moveOverlay == nil {
		return nil
	}
	model := m.moveOverlay.detail
	if model == nil {
		return nil
	}
	if msg.err != nil {
		model.SetError(msg.err)
		if m.command != nil {
			m.setStatus("Bullet detail error: " + msg.err.Error())
		}
		return nil
	}
	if msg.entry == nil {
		model.SetError(fmt.Errorf("entry not available"))
		return nil
	}
	model.SetEntry(msg.entry)
	return nil
}
