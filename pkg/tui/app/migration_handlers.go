package app

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/collection"
	viewmodel "tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/tui/components/collectionnav"
	"tableflip.dev/bujo/pkg/tui/components/command"
	"tableflip.dev/bujo/pkg/tui/events"
)

func (m *Model) closeMigrateOverlay() tea.Cmd {
	return m.closeMigrateOverlayWithStatus("")
}

func (m *Model) closeMigrateOverlayWithStatus(status string) tea.Cmd {
	if !m.migrateVisible {
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
		m.overlayStack.Close(overlayKindMigrate)
	}
	m.migrateOverlay = nil
	m.migrateVisible = false
	m.migrateWindow = migrationWindow{}
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

func (m *Model) showMigrateOverlay(arg string) (tea.Cmd, string) {
	if m.command == nil {
		return nil, "noop"
	}
	m.ensureOverlayStack()
	if m.service == nil {
		m.setStatus("Migration unavailable: service offline")
		return nil, "error"
	}
	if m.migrateVisible {
		return m.closeMigrateOverlay(), "closed"
	}
	now := m.now()
	if !m.today.IsZero() {
		now = now.In(m.today.Location())
		if now.Before(m.today) {
			now = m.today
		}
	}
	window, err := resolveMigrationWindow(now, arg)
	if err != nil {
		m.setStatus("Migration: " + err.Error())
		return nil, "error"
	}
	ctx := context.Background()
	metas, err := m.service.CollectionsMeta(ctx, "")
	if err != nil {
		m.setStatus("Migration unavailable: " + err.Error())
		return nil, "error"
	}
	parsedRoots := viewmodel.BuildTree(metas)
	futureRoots := futureCollectionsFromMetas(metas, now)
	targetRoots := filterMoveCollections(parsedRoots)
	targetRoots = appendNewCollectionOption(targetRoots)
	targetRoots = includeNextMonthCollection(targetRoots, now)
	candidates, err := m.service.MigrationCandidates(ctx, window.Since, window.Until)
	if err != nil {
		m.setStatus("Migration unavailable: " + err.Error())
		return nil, "error"
	}
	data := buildMigrationData(now, candidates, parsedRoots)
	futureNav := collectionnav.NewModel(futureRoots)
	futureNav.SetBlurOnSelect(false)
	targetNav := collectionnav.NewModel(targetRoots)
	targetNav.SetBlurOnSelect(false)
	overlay := newMigrationOverlay(data, window, futureNav, targetNav, m.dump)
	overlay.SetSize(m.width, maxInt(1, m.height-1))

	if data.IsEmpty() && m.command != nil {
		m.setStatus("Migration inbox empty")
	}

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
	if m.newCollectionVisible {
		if cmd := m.closeNewCollectionOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.migrateVisible {
		if cmd := m.closeMigrateOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	placement := command.OverlayPlacement{Fullscreen: true}
	if cmd := m.overlayStack.Open(overlayKindMigrate, overlay, placement); cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.migrateOverlay = overlay
	m.migrateVisible = true
	m.migrateWindow = window
	_ = m.dropFocusKind(focusKindCommand)
	m.pushFocus(focusTarget{kind: focusKindOverlay, overlay: overlayKindMigrate})
	cmds = append(cmds, m.blurJournalPanes()...)
	if focus := m.overlayStack.Focus(); focus != nil {
		cmds = append(cmds, focus)
	}
	if len(cmds) == 0 {
		return nil, "opened"
	}
	return tea.Batch(cmds...), "opened"
}

func (m *Model) handleMigrationMoveRequest(msg events.MoveBulletRequestMsg) tea.Cmd {
	if !m.migrateVisible || m.migrateOverlay == nil {
		return nil
	}
	bulletID := strings.TrimSpace(msg.Bullet.ID)
	if bulletID == "" {
		return nil
	}
	targetRef, exists, ok := m.migrateOverlay.TargetSelection()
	if !ok {
		if m.command != nil {
			m.setStatus("Move unavailable: select a destination")
		}
		return nil
	}
	targetPath := strings.TrimSpace(resolvedCollectionPath(targetRef))
	if targetPath == "" {
		targetPath = strings.TrimSpace(targetRef.Name)
	}
	if targetPath == "" {
		if m.command != nil {
			m.setStatus("Move unavailable: select a destination")
		}
		return nil
	}
	origin := strings.TrimSpace(msg.Collection.ID)
	if origin == "" {
		origin = strings.TrimSpace(msg.Bullet.Note)
	}
	label := strings.TrimSpace(msg.Bullet.Label)
	if label == "" {
		label = bulletID
	}
	if targetPath == origin {
		status := "Kept " + label
		return m.removeMigrationBullet(bulletID, status)
	}
	if m.service == nil {
		if m.command != nil {
			m.setStatus("Move unavailable: service offline")
		}
		return nil
	}
	ctx := context.Background()
	if !exists {
		targetType := targetRef.Type
		if targetType == "" {
			targetType = collection.TypeGeneric
		}
		if err := m.service.EnsureCollectionOfType(ctx, targetPath, targetType); err != nil {
			if m.command != nil {
				m.setStatus("Move failed: " + err.Error())
			}
			return nil
		}
	}
	clone, err := m.service.Move(ctx, bulletID, targetPath)
	if err != nil {
		if m.command != nil {
			m.setStatus("Move failed: " + err.Error())
		}
		return nil
	}
	if strings.TrimSpace(label) == "" && clone != nil {
		label = strings.TrimSpace(clone.Message)
		if label == "" {
			label = bulletID
		}
	}
	destination := targetRef.Label()
	if strings.TrimSpace(destination) == "" {
		destination = targetPath
	}
	status := "Moved " + label + " to " + destination
	var cmds []tea.Cmd
	if cmd := m.removeMigrationBullet(bulletID, status); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if sync := m.collectionSyncCmd(targetPath); sync != nil {
		cmds = append(cmds, sync)
	}
	if origin != "" && origin != targetPath {
		if sync := m.collectionSyncCmd(origin); sync != nil {
			cmds = append(cmds, sync)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) handleMigrationMoveFuture(msg events.BulletMoveFutureMsg) tea.Cmd {
	if !m.migrateVisible || m.migrateOverlay == nil {
		return nil
	}
	bulletID := strings.TrimSpace(msg.Bullet.ID)
	if bulletID == "" {
		return nil
	}
	targetRef, exists, ok := m.migrateOverlay.FutureSelection()
	if !ok {
		targetRef = events.CollectionRef{ID: "Future", Name: "Future", Type: collection.TypeMonthly}
		exists = true
	}
	targetPath := strings.TrimSpace(resolvedCollectionPath(targetRef))
	if targetPath == "" {
		targetPath = "Future"
	}
	origin := strings.TrimSpace(msg.Collection.ID)
	if origin == "" {
		origin = strings.TrimSpace(msg.Bullet.Note)
	}
	label := strings.TrimSpace(msg.Bullet.Label)
	if label == "" {
		label = bulletID
	}
	if targetPath == origin {
		status := "Kept " + label
		return m.removeMigrationBullet(bulletID, status)
	}
	if m.service == nil {
		if m.command != nil {
			m.setStatus("Move unavailable: service offline")
		}
		return nil
	}
	ctx := context.Background()
	if !exists {
		targetType := targetRef.Type
		if strings.HasPrefix(targetPath, "Future/") {
			if targetType == "" {
				targetType = collection.TypeGeneric
			}
		} else if targetType == "" {
			if targetPath == "Future" {
				targetType = collection.TypeMonthly
			} else {
				targetType = collection.TypeGeneric
			}
		}
		if err := m.service.EnsureCollectionOfType(ctx, targetPath, targetType); err != nil {
			if m.command != nil {
				m.setStatus("Move failed: " + err.Error())
			}
			return nil
		}
	}
	clone, err := m.service.Move(ctx, bulletID, targetPath)
	if err != nil {
		if m.command != nil {
			m.setStatus("Move failed: " + err.Error())
		}
		return nil
	}
	if strings.TrimSpace(label) == "" && clone != nil {
		label = strings.TrimSpace(clone.Message)
		if label == "" {
			label = bulletID
		}
	}
	dest := targetRef.Label()
	if strings.TrimSpace(dest) == "" {
		dest = targetPath
	}
	status := "Moved " + label + " to " + dest
	var cmds []tea.Cmd
	if cmd := m.removeMigrationBullet(bulletID, status); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if sync := m.collectionSyncCmd(targetPath); sync != nil {
		cmds = append(cmds, sync)
	}
	if origin != "" && origin != targetPath {
		if sync := m.collectionSyncCmd(origin); sync != nil {
			cmds = append(cmds, sync)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) handleMigrationKeep(msg events.BulletSelectMsg) tea.Cmd {
	if !m.migrateVisible || m.migrateOverlay == nil {
		return nil
	}
	if !msg.Exists {
		return nil
	}
	bulletID := strings.TrimSpace(msg.Bullet.ID)
	if bulletID == "" {
		return nil
	}
	label := strings.TrimSpace(msg.Bullet.Label)
	if label == "" {
		label = bulletID
	}
	status := "Kept " + label
	return m.removeMigrationBullet(bulletID, status)
}

func (m *Model) removeMigrationBullet(id, status string) tea.Cmd {
	id = strings.TrimSpace(id)
	if id == "" {
		if status != "" && m.command != nil {
			m.setStatus(status)
		}
		return nil
	}
	if !m.migrateVisible || m.migrateOverlay == nil {
		if status != "" && m.command != nil {
			m.setStatus(status)
		}
		return nil
	}
	m.migrateOverlay.RemoveBullet(id)
	if m.migrateOverlay.IsEmpty() {
		final := status
		if strings.TrimSpace(final) == "" {
			final = "Migration complete"
		}
		return m.closeMigrateOverlayWithStatus(final)
	}
	if status != "" && m.command != nil {
		m.setStatus(status)
	}
	if cmd := m.migrateOverlay.FocusDetail(); cmd != nil {
		return cmd
	}
	return nil
}

func (m *Model) handleMigrationCollectionSelect(msg events.CollectionSelectMsg) tea.Cmd {
	if m.migrateOverlay == nil {
		return nil
	}
	if msg.Component == migrateTargetNavID && strings.EqualFold(strings.TrimSpace(msg.Collection.ID), migrationNewCollectionID) {
		return m.startMigrationNewCollectionPrompt()
	}
	section, bulletRow, item, ok := m.migrateOverlay.CurrentMigrationSelection()
	if !ok || item == nil || item.Candidate.Entry == nil {
		return m.migrateOverlay.FocusDetail()
	}
	label := strings.TrimSpace(bulletRow.Label)
	if label == "" {
		label = strings.TrimSpace(item.Candidate.Entry.Message)
	}
	if label == "" {
		label = bulletRow.ID
	}
	sectionRef := events.CollectionViewRef{
		ID:       section.ID,
		Title:    section.Title,
		Subtitle: section.Subtitle,
	}
	bulletRef := events.BulletRef{
		ID:        bulletRow.ID,
		Label:     label,
		Note:      item.SectionID,
		Bullet:    bulletRow.Bullet,
		Signifier: bulletRow.Signifier,
	}
	switch msg.Component {
	case migrateFutureNavID:
		return m.handleMigrationMoveFuture(events.BulletMoveFutureMsg{
			Component:  migrateDetailID,
			Collection: sectionRef,
			Bullet:     bulletRef,
		})
	case migrateTargetNavID:
		return m.handleMigrationMoveRequest(events.MoveBulletRequestMsg{
			Component:  migrateDetailID,
			Collection: sectionRef,
			Bullet:     bulletRef,
		})
	default:
		return m.migrateOverlay.FocusDetail()
	}
}

func (m *Model) startMigrationNewCollectionPrompt() tea.Cmd {
	if m.migrateOverlay == nil {
		return nil
	}
	if m.command != nil {
		m.setStatus("Enter a name for the new collection")
	}
	return m.migrateOverlay.BeginNewCollectionPrompt()
}

func (m *Model) handleMigrationCreateCollection(name string) tea.Cmd {
	name = strings.TrimSpace(name)
	if name == "" {
		if m.command != nil {
			m.setStatus("Collection name cannot be empty")
		}
		if m.migrateOverlay != nil {
			return m.migrateOverlay.FocusDetail()
		}
		return nil
	}
	if m.service == nil {
		if m.command != nil {
			m.setStatus("Create failed: service offline")
		}
		if m.migrateOverlay != nil {
			return m.migrateOverlay.FocusDetail()
		}
		return nil
	}
	ctx := context.Background()
	if err := m.service.EnsureCollectionOfType(ctx, name, collection.TypeGeneric); err != nil {
		if m.command != nil {
			m.setStatus("Create failed: " + err.Error())
		}
		if m.migrateOverlay != nil {
			return m.migrateOverlay.FocusDetail()
		}
		return nil
	}
	cmds := []tea.Cmd{}
	if cmd := m.collectionSyncCmd(name); cmd != nil {
		cmds = append(cmds, cmd)
	}
	_, bulletRow, item, ok := m.migrateOverlay.CurrentMigrationSelection()
	if !ok || item == nil || item.Candidate.Entry == nil {
		if focus := m.migrateOverlay.FocusDetail(); focus != nil {
			cmds = append(cmds, focus)
		}
		if len(cmds) == 0 {
			return nil
		}
		return tea.Batch(cmds...)
	}
	bulletID := strings.TrimSpace(item.Candidate.Entry.ID)
	if bulletID == "" {
		if focus := m.migrateOverlay.FocusDetail(); focus != nil {
			cmds = append(cmds, focus)
		}
		if len(cmds) == 0 {
			return nil
		}
		return tea.Batch(cmds...)
	}
	clone, err := m.service.Move(ctx, bulletID, name)
	if err != nil {
		if m.command != nil {
			m.setStatus("Move failed: " + err.Error())
		}
		if focus := m.migrateOverlay.FocusDetail(); focus != nil {
			cmds = append(cmds, focus)
		}
		if len(cmds) == 0 {
			return nil
		}
		return tea.Batch(cmds...)
	}
	label := strings.TrimSpace(bulletRow.Label)
	if label == "" && clone != nil {
		label = strings.TrimSpace(clone.Message)
	}
	if label == "" {
		label = bulletID
	}
	status := "Moved " + label + " to " + name
	if cmd := m.removeMigrationBullet(bulletID, status); cmd != nil {
		cmds = append(cmds, cmd)
	}
	origin := strings.TrimSpace(item.Candidate.Entry.Collection)
	if origin != "" && !strings.EqualFold(origin, name) {
		if cmd := m.collectionSyncCmd(origin); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.command != nil {
		m.setStatus("Created " + name + " · moved " + label)
	}
	if focus := m.migrateOverlay.FocusDetail(); focus != nil {
		cmds = append(cmds, focus)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}
