package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/tui/components/bulletdetail"
	"tableflip.dev/bujo/pkg/tui/components/collectionnav"
	"tableflip.dev/bujo/pkg/tui/components/command"
	"tableflip.dev/bujo/pkg/tui/events"
)

// moveOverlayConfig bundles dependencies for the move bullet overlay.
type moveOverlayConfig struct {
	detail       *bulletdetail.Model
	nav          *collectionnav.Model
	bulletID     string
	collectionID string
	label        string
	status       string
	initialRef   events.CollectionRef
	futureOnly   bool
	navOnRight   bool
}

func (m *Model) handleMoveBulletRequest(msg events.MoveBulletRequestMsg) tea.Cmd {
	bulletID := strings.TrimSpace(msg.Bullet.ID)
	if bulletID == "" {
		if m.command != nil {
			m.setStatus("Move unavailable: missing bullet ID")
		}
		return nil
	}
	if m.service == nil {
		if m.command != nil {
			m.setStatus("Move unavailable: service offline")
		}
		return nil
	}
	if m.journalCache == nil {
		if m.command != nil {
			m.setStatus("Move unavailable: journal cache offline")
		}
		return nil
	}
	collectionID := strings.TrimSpace(msg.Collection.ID)
	if collectionID == "" {
		collectionID = strings.TrimSpace(msg.Bullet.Note)
	}
	if collectionID == "" {
		if m.command != nil {
			m.setStatus("Move unavailable: missing collection context")
		}
		return nil
	}
	snapshot := m.journalCache.Snapshot()
	if len(snapshot.Collections) == 0 || len(snapshot.Sections) == 0 {
		if m.command != nil {
			m.setStatus("Move unavailable: no collections")
		}
		return nil
	}
	if source := findBulletInSnapshot(snapshot.Sections, collectionID, bulletID); source != nil && source.Locked {
		if m.command != nil {
			m.setStatus("Move unavailable: bullet is locked")
		}
		return nil
	}
	trimmedCollections := filterMoveCollections(snapshot.Collections)
	trimmedCollections = appendNewCollectionOption(trimmedCollections)
	if len(trimmedCollections) == 0 {
		if m.command != nil {
			m.setStatus("Move unavailable: no target collections")
		}
		return nil
	}
	title := strings.TrimSpace(msg.Collection.Title)
	if title == "" {
		title = collectionID
	}
	detailModel := bulletdetail.New(title, msg.Bullet.Label, collectionID, msg.Bullet.Note)

	nav := collectionnav.NewModel(trimmedCollections)
	nav.SetBlurOnSelect(false)
	label := strings.TrimSpace(msg.Bullet.Label)
	if label == "" {
		label = bulletID
	}
	cfg := moveOverlayConfig{
		detail:       detailModel,
		nav:          nav,
		bulletID:     bulletID,
		collectionID: collectionID,
		label:        label,
		status:       "Choose destination for " + label,
		initialRef:   events.CollectionRef{ID: collectionID, Name: title},
		futureOnly:   false,
		navOnRight:   true,
	}
	return m.openMoveOverlay(cfg)
}

func (m *Model) openMoveOverlay(cfg moveOverlayConfig) tea.Cmd {
	if cfg.bulletID == "" {
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
	if m.newCollectionVisible {
		if cmd := m.closeNewCollectionOverlay(); cmd != nil {
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
	if cfg.detail != nil {
		cfg.detail.SetLoading(true)
	}
	if cfg.nav != nil {
		cfg.nav.SetID(moveNavID)
	}
	mOverlay := newMovebulletOverlay(cfg.detail, cfg.nav, cfg.navOnRight, m.dump)
	placement := command.OverlayPlacement{Fullscreen: true}
	if cmd := m.overlayStack.Open(overlayKindMove, mOverlay, placement); cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.moveOverlay = mOverlay
	m.moveVisible = true
	m.moveBulletID = cfg.bulletID
	m.moveCollectionID = cfg.collectionID
	m.moveFutureOnly = cfg.futureOnly
	reqID := fmt.Sprintf("%s@%d", cfg.bulletID, time.Now().UnixNano())
	m.moveLoadID = reqID
	_ = m.dropFocusKind(focusKindCommand)
	m.pushFocus(focusTarget{kind: focusKindOverlay, overlay: overlayKindMove})
	cmds = append(cmds, m.blurJournalPanes()...)
	if focusCmd := m.overlayStack.Focus(); focusCmd != nil {
		cmds = append(cmds, focusCmd)
	}
	if cfg.nav != nil {
		if cfg.initialRef.ID != "" || cfg.initialRef.Name != "" {
			if cmd := cfg.nav.SelectCollection(cfg.initialRef); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	if m.command != nil {
		label := cfg.label
		if label == "" {
			label = cfg.bulletID
		}
		status := strings.TrimSpace(cfg.status)
		if status == "" {
			status = "Choose destination for " + label
		}
		m.setStatus(status)
	}
	if loadCmd := m.loadBulletDetail(cfg.collectionID, cfg.bulletID, reqID); loadCmd != nil {
		cmds = append(cmds, loadCmd)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) handleBulletMoveFuture(msg events.BulletMoveFutureMsg) tea.Cmd {
	if m.migrateVisible {
		return nil
	}
	bulletID := strings.TrimSpace(msg.Bullet.ID)
	if bulletID == "" {
		if m.command != nil {
			m.setStatus("Move unavailable: missing bullet ID")
		}
		return nil
	}
	if m.service == nil {
		if m.command != nil {
			m.setStatus("Move unavailable: service offline")
		}
		return nil
	}
	if m.journalCache == nil {
		if m.command != nil {
			m.setStatus("Move unavailable: journal cache offline")
		}
		return nil
	}
	collectionID := strings.TrimSpace(msg.Collection.ID)
	if collectionID == "" {
		collectionID = strings.TrimSpace(msg.Bullet.Note)
	}
	if collectionID == "" {
		if m.command != nil {
			m.setStatus("Move unavailable: missing collection context")
		}
		return nil
	}
	snapshot := m.journalCache.Snapshot()
	if len(snapshot.Collections) == 0 || len(snapshot.Sections) == 0 {
		if m.command != nil {
			m.setStatus("Move unavailable: no collections")
		}
		return nil
	}
	if source := findBulletInSnapshot(snapshot.Sections, collectionID, bulletID); source != nil && source.Locked {
		if m.command != nil {
			m.setStatus("Move unavailable: bullet is locked")
		}
		return nil
	}
	ctx := context.Background()
	now := m.today
	if now.IsZero() {
		now = m.now()
	}
	roots, err := m.futureMoveCollections(ctx, now)
	if err != nil {
		if m.command != nil {
			m.setStatus("Move unavailable: " + err.Error())
		}
		return nil
	}
	if len(roots) == 0 {
		if m.command != nil {
			m.setStatus("Move unavailable: Future view not ready")
		}
		return nil
	}
	title := strings.TrimSpace(msg.Collection.Title)
	if title == "" {
		title = collectionID
	}
	detailModel := bulletdetail.New(title, msg.Bullet.Label, collectionID, msg.Bullet.Note)
	nav := collectionnav.NewModel(roots)
	nav.SetBlurOnSelect(false)
	label := strings.TrimSpace(msg.Bullet.Label)
	if label == "" {
		label = bulletID
	}
	initial := events.CollectionRef{ID: "Future", Name: "Future"}
	if strings.HasPrefix(collectionID, "Future/") {
		initial.ID = collectionID
		name := strings.TrimPrefix(collectionID, "Future/")
		if name == "" {
			name = collectionID
		}
		initial.Name = name
	} else if collectionID == "Future" {
		initial.ID = collectionID
		initial.Name = "Future"
	}
	cfg := moveOverlayConfig{
		detail:       detailModel,
		nav:          nav,
		bulletID:     bulletID,
		collectionID: collectionID,
		label:        label,
		status:       "Choose Future destination for " + label,
		initialRef:   initial,
		futureOnly:   true,
		navOnRight:   false,
	}
	return m.openMoveOverlay(cfg)
}

func (m *Model) handleMoveSelection(msg events.CollectionSelectMsg) tea.Cmd {
	if !m.moveVisible {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(msg.Collection.ID), newCollectionOptionID) {
		return m.startMoveNewCollectionPrompt()
	}
	target := resolvedCollectionPath(msg.Collection)
	collectionLabel := strings.TrimSpace(msg.Collection.Label())
	if strings.EqualFold(strings.TrimSpace(target), newCollectionOptionID) ||
		strings.EqualFold(strings.TrimSpace(msg.Collection.Name), newCollectionOptionLabel) ||
		strings.EqualFold(collectionLabel, newCollectionOptionLabel) ||
		strings.Contains(strings.ToLower(target), newCollectionOptionID) {
		return m.startMoveNewCollectionPrompt()
	}
	if target == "" {
		return nil
	}
	bulletID := strings.TrimSpace(m.moveBulletID)
	if bulletID == "" {
		return m.closeMoveOverlayWithStatus("Move unavailable: no bullet selected")
	}
	if target == strings.TrimSpace(m.moveCollectionID) {
		return m.closeMoveOverlayWithStatus("Bullet already in selected collection")
	}
	if m.service == nil {
		if m.command != nil {
			m.setStatus("Move failed: service offline")
		}
		return nil
	}
	ctx := context.Background()
	if m.moveFutureOnly && !msg.Exists {
		targetType := msg.Collection.Type
		if strings.HasPrefix(target, "Future/") {
			targetType = collection.TypeGeneric
		}
		if targetType == "" {
			switch {
			case strings.HasPrefix(target, "Future/"):
				targetType = collection.TypeGeneric
			case target == "Future":
				targetType = collection.TypeMonthly
			default:
				targetType = collection.TypeGeneric
			}
		}
		if err := m.service.EnsureCollectionOfType(ctx, target, targetType); err != nil {
			if m.command != nil {
				m.setStatus("Move failed: " + err.Error())
			}
			return nil
		}
	}
	clone, err := m.service.Move(ctx, bulletID, target)
	if err != nil {
		if m.command != nil {
			m.setStatus("Move failed: " + err.Error())
		}
		return nil
	}
	label := collectionLabel
	if label == "" {
		label = target
	}
	var cmds []tea.Cmd
	if m.journalNav != nil {
		ref := events.CollectionRef{ID: target}
		if idx := strings.LastIndex(target, "/"); idx >= 0 {
			ref.ParentID = target[:idx]
			ref.Name = strings.TrimSpace(target[idx+1:])
		} else {
			ref.Name = target
		}
		if cmd := m.journalNav.SelectCollection(ref); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if cache := m.journalCache; cache != nil {
		if cmd := m.collectionSyncCmd(target); cmd != nil {
			cmds = append(cmds, cmd)
		}
		origin := strings.TrimSpace(m.moveCollectionID)
		if origin == "" {
			origin = clone.Collection
		}
		if origin != "" {
			if cmd := m.collectionSyncCmd(origin); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	if cmd := m.closeMoveOverlayWithStatus("Moved bullet to " + label); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if !msg.Exists {
		if snap := m.snapshotSyncCmd(); snap != nil {
			cmds = append(cmds, snap)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) startMoveNewCollectionPrompt() tea.Cmd {
	if m.moveOverlay == nil {
		return nil
	}
	if m.command != nil {
		m.setStatus("Enter a name for the new collection")
	}
	return m.moveOverlay.BeginNewCollectionPrompt()
}

func (m *Model) handleMoveCreateCollection(name string) tea.Cmd {
	name = strings.TrimSpace(name)
	if name == "" {
		if m.command != nil {
			m.setStatus("Collection name cannot be empty")
		}
		if m.moveOverlay != nil {
			return m.moveOverlay.BeginNewCollectionPrompt()
		}
		return nil
	}
	if m.service == nil {
		if m.command != nil {
			m.setStatus("Create failed: service offline")
		}
		if m.moveOverlay != nil {
			return m.moveOverlay.FocusNav()
		}
		return nil
	}
	bulletID := strings.TrimSpace(m.moveBulletID)
	if bulletID == "" {
		return m.closeMoveOverlayWithStatus("Move unavailable: no bullet selected")
	}
	ctx := context.Background()
	if err := m.service.EnsureCollectionOfType(ctx, name, collection.TypeGeneric); err != nil {
		if m.command != nil {
			m.setStatus("Create failed: " + err.Error())
		}
		if m.moveOverlay != nil {
			return m.moveOverlay.BeginNewCollectionPrompt()
		}
		return nil
	}
	clone, err := m.service.Move(ctx, bulletID, name)
	if err != nil {
		if m.command != nil {
			m.setStatus("Move failed: " + err.Error())
		}
		if m.moveOverlay != nil {
			return m.moveOverlay.BeginNewCollectionPrompt()
		}
		return nil
	}

	target := strings.TrimSpace(name)
	if target == "" {
		target = name
	}
	var cmds []tea.Cmd
	if m.journalNav != nil {
		ref := events.CollectionRef{ID: target}
		if idx := strings.LastIndex(target, "/"); idx >= 0 {
			ref.ParentID = strings.TrimSpace(target[:idx])
			ref.Name = strings.TrimSpace(target[idx+1:])
		} else {
			ref.Name = target
		}
		if cmd := m.journalNav.SelectCollection(ref); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if cache := m.journalCache; cache != nil {
		if cmd := m.collectionSyncCmd(target); cmd != nil {
			cmds = append(cmds, cmd)
		}
		origin := strings.TrimSpace(m.moveCollectionID)
		if origin == "" && clone != nil {
			origin = strings.TrimSpace(clone.Collection)
		}
		if origin != "" && !strings.EqualFold(origin, target) {
			if cmd := m.collectionSyncCmd(origin); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	status := "Created " + target + " · moved bullet"
	if cmd := m.closeMoveOverlayWithStatus(status); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if snap := m.snapshotSyncCmd(); snap != nil {
		cmds = append(cmds, snap)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) closeMoveOverlay() tea.Cmd {
	return m.closeMoveOverlayWithStatus("")
}

func (m *Model) closeMoveOverlayWithStatus(status string) tea.Cmd {
	if !m.moveVisible {
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
		m.overlayStack.Close(overlayKindMove)
	}
	m.moveVisible = false
	m.moveOverlay = nil
	m.moveBulletID = ""
	m.moveCollectionID = ""
	m.moveFutureOnly = false
	m.moveLoadID = ""
	_, _ = m.popFocusKind(focusKindOverlay)
	if cmd := m.restoreFocusAfterOverlay(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if status != "" && m.command != nil {
		m.setStatus(status)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}
