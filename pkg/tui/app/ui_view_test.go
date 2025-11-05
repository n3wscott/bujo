package teaui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/tui/components/detail"
	migrationview "tableflip.dev/bujo/pkg/tui/views/migration"
	wizardview "tableflip.dev/bujo/pkg/tui/views/wizard"
)

func newTestEntry(id, collectionID, message string, created time.Time) *entry.Entry {
	return &entry.Entry{
		ID:         id,
		Collection: collectionID,
		Message:    message,
		Bullet:     glyph.Task,
		Signifier:  glyph.None,
		Created:    entry.Timestamp{Time: created},
	}
}

func TestViewNormalModeRendersSelectionAndDetail(t *testing.T) {
	m := New(nil)
	m.termWidth = 96
	m.termHeight = 28
	m.applySizes()

	now := time.Date(2025, time.March, 3, 12, 0, 0, 0, time.UTC)
	metas := []collection.Meta{
		{Name: "Future", Type: collection.TypeMonthly},
		{Name: "Future/March 2026", Type: collection.TypeDaily},
		{Name: "Projects", Type: collection.TypeGeneric},
	}
	items := m.buildCollectionItems(metas, "Future/March 2026", now)
	m.colList.SetItems(items)

	idx := indexForResolved(items, "Future/March 2026")
	if idx < 0 {
		t.Fatalf("resolved collection not found in items")
	}
	m.colList.Select(idx)
	m.focus = 1
	m.updateFocusHeaders()
	m.updateBottomContext()

	task := newTestEntry("entry-1", "Future/March 2026", "Follow up on planning", now)
	m.detailState.SetSections([]detail.Section{
		{
			CollectionID:   "Future/March 2026",
			CollectionName: "Future/March 2026",
			ResolvedName:   "Future/March 2026",
			Entries:        []*entry.Entry{task},
		},
	})
	m.detailState.SetActive("Future/March 2026", "entry-1")

	view := stripANSI(m.View())
	if !strings.Contains(view, "▾ Future") {
		t.Fatalf("expected Future collection to appear expanded; view=%q", view)
	}
	if !strings.Contains(view, "Future/March 2026") {
		t.Fatalf("expected detail header for resolved collection; view=%q", view)
	}
	if !strings.Contains(view, "→  ⦁ Follow up on planning") {
		t.Fatalf("expected active entry marker in detail pane; view=%q", view)
	}
	if !strings.Contains(view, "Entries · j/k move") {
		t.Fatalf("expected contextual entries help in bottom bar")
	}
}

func TestViewCommandModeDisplaysSuggestions(t *testing.T) {
	m := New(nil)
	m.termWidth = 96
	m.termHeight = 28
	m.applySizes()

	now := time.Date(2025, time.March, 3, 12, 0, 0, 0, time.UTC)
	metas := []collection.Meta{
		{Name: "Future", Type: collection.TypeMonthly},
		{Name: "Future/March 2026", Type: collection.TypeDaily},
	}
	items := m.buildCollectionItems(metas, "Future/March 2026", now)
	m.colList.SetItems(items)
	if idx := indexForResolved(items, "Future/March 2026"); idx >= 0 {
		m.colList.Select(idx)
	}
	m.detailState.SetSections(nil)

	var cmds []tea.Cmd
	m.enterCommandMode(&cmds)
	m.input.SetValue("to")
	m.input.CursorEnd()
	m.bottom.UpdateCommandInput(m.input.Value(), m.input.View())
	m.bottom.StepSuggestion(1)
	view := stripANSI(m.View())
	if !strings.Contains(view, "→ today  Jump to Today collection") {
		t.Fatalf("expected :today suggestion in command footer; view=%q", view)
	}
	if !strings.Contains(view, ":to") {
		t.Fatalf("expected command line to include typed input :to; view=%q", view)
	}
}

func TestCommandModeStaysAnchoredAfterDetailScroll(t *testing.T) {
	m := New(nil)
	m.termWidth = 96
	m.termHeight = 30
	m.applySizes()

	height := m.detailHeight
	if height <= 0 {
		t.Fatalf("unexpected detail height %d", height)
	}

	const totalEntries = 120
	entries := make([]*entry.Entry, 0, totalEntries)
	for i := 0; i < totalEntries; i++ {
		entries = append(entries, &entry.Entry{
			ID:      fmt.Sprintf("entry-%03d", i),
			Message: fmt.Sprintf("Task #%03d", i),
			Bullet:  glyph.Task,
		})
	}
	m.detailState.SetSections([]detail.Section{
		{
			CollectionID:   "Inbox",
			CollectionName: "Inbox",
			ResolvedName:   "Inbox",
			Entries:        entries,
		},
	})
	m.detailState.SetActive("Inbox", entries[0].ID)

	for i := 0; i < height*2; i++ {
		m.detailState.MoveEntry(1)
	}

	var cmds []tea.Cmd
	m.enterCommandMode(&cmds)

	m.input.SetValue("to")
	m.input.CursorEnd()
	m.bottom.UpdateCommandInput(m.input.Value(), m.input.View())
	m.bottom.StepSuggestion(1)

	view := stripANSI(m.View())
	lines := strings.Split(view, "\n")
	if len(lines) == 0 {
		t.Fatalf("view unexpectedly empty")
	}
	last := lines[len(lines)-1]
	if !strings.Contains(last, ":to") {
		t.Fatalf("expected command prompt on final line, got %q", last)
	}
	first := strings.TrimSpace(lines[0])
	if strings.HasPrefix(first, ":") {
		t.Fatalf("expected main content before command lines, got %q", lines[0])
	}
}

func TestCommandModeAnchorsAfterColonKeyWithScroll(t *testing.T) {
	m := New(nil)
	m.termWidth = 96
	m.termHeight = 30
	m.applySizes()

	height := m.detailHeight
	if height <= 0 {
		t.Fatalf("unexpected detail height %d", height)
	}

	const totalEntries = 200
	entries := make([]*entry.Entry, 0, totalEntries)
	for i := 0; i < totalEntries; i++ {
		entries = append(entries, &entry.Entry{
			ID:      fmt.Sprintf("entry-%03d", i),
			Message: fmt.Sprintf("Task #%03d", i),
			Bullet:  glyph.Task,
		})
	}
	m.detailState.SetSections([]detail.Section{
		{
			CollectionID:   "Inbox",
			CollectionName: "Inbox",
			ResolvedName:   "Inbox",
			Entries:        entries,
		},
	})
	m.detailState.SetActive("Inbox", entries[0].ID)
	m.focus = 1

	for i := 0; i < height*2; i++ {
		next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		m = assertAppModel(t, next)
		m = drainAppCommands(t, m, cmd)
	}

	colon := tea.KeyPressMsg{Text: ":", Code: ':'}
	next, cmd := m.Update(colon)
	m = assertAppModel(t, next)
	m = drainAppCommands(t, m, cmd)

	view := stripANSI(m.View())
	lines := strings.Split(view, "\n")
	if len(lines) == 0 {
		t.Fatalf("view unexpectedly empty after entering command mode")
	}
	last := strings.TrimSpace(lines[len(lines)-1])
	if !strings.HasPrefix(last, ":") {
		t.Fatalf("expected prompt on final line after colon key, got %q", last)
	}
	first := strings.TrimSpace(lines[0])
	if strings.HasPrefix(first, ":") {
		t.Fatalf("expected main content before command lines, got %q", lines[0])
	}
}

func drainAppCommands(t *testing.T, m *Model, cmds ...tea.Cmd) *Model {
	queue := append([]tea.Cmd(nil), cmds...)
	for len(queue) > 0 {
		cmd := queue[0]
		queue = queue[1:]
		if cmd == nil {
			continue
		}
		msg := cmd()
		switch v := msg.(type) {
		case tea.BatchMsg:
			queue = append(queue, []tea.Cmd(v)...)
		default:
			next, nextCmd := m.Update(v)
			m = assertAppModel(t, next)
			if nextCmd != nil {
				queue = append(queue, nextCmd)
			}
		}
	}
	return m
}

func assertAppModel(t *testing.T, model tea.Model) *Model {
	t.Helper()
	m, ok := model.(*Model)
	if !ok {
		t.Fatalf("unexpected model type %T", model)
	}
	return m
}

func TestViewWizardModeShowsOverlay(t *testing.T) {
	m := New(nil)
	m.termWidth = 90
	m.termHeight = 26
	m.applySizes()

	m.wizard.Active = true
	m.wizard.ParentOptions = []string{"Future"}
	m.wizard.ParentIndex = 0
	m.wizard.Step = wizardview.StepParent
	m.setMode(modeCollectionWizard)

	view := stripANSI(m.View())
	if !strings.Contains(view, "New Collection") {
		t.Fatalf("expected wizard overlay title; view=%q", view)
	}
	if !strings.Contains(view, "Parent") {
		t.Fatalf("expected wizard to list parent step; view=%q", view)
	}
	if !strings.Contains(view, "→ <root>") {
		t.Fatalf("expected current parent selection marker; view=%q", view)
	}
}

func TestViewReportModeShowsReportOverlay(t *testing.T) {
	m := New(nil)
	m.termWidth = 90
	m.termHeight = 26
	m.applySizes()

	now := time.Date(2025, time.July, 9, 10, 0, 0, 0, time.UTC)
	section := app.ReportSection{
		Collection: "Projects",
		Entries: []app.ReportItem{
			{
				Entry:       newTestEntry("completed-1", "Projects", "Ship release", now),
				Completed:   true,
				CompletedAt: now.Add(-48 * time.Hour),
			},
		},
	}
	m.report.SetData("3d", now.Add(-72*time.Hour), now, len(section.Entries), []app.ReportSection{section})
	m.setMode(modeReport)

	view := stripANSI(m.View())
	if !strings.Contains(view, "Projects") {
		t.Fatalf("expected report overlay to include collection header; view=%q", view)
	}
	if !strings.Contains(view, "Ship release") {
		t.Fatalf("expected report overlay to include entry line; view=%q", view)
	}
}

func TestMigrationViewRendersCandidates(t *testing.T) {
	m := New(nil)
	m.termWidth = 96
	m.termHeight = 28
	m.applySizes()

	base := time.Date(2025, time.November, 12, 9, 0, 0, 0, time.UTC)
	parent := newTestEntry("parent", "Inbox", "Project Kickoff", base.Add(-8*24*time.Hour))
	task := newTestEntry("task-1", "Inbox", "Finalize agenda", base.Add(-5*24*time.Hour))
	task.ParentID = parent.ID
	m.migration = migrationview.New(m.theme, relativeTime)
	m.migration.Active = true
	m.migration.Label = "1 week"
	m.migration.Items = []migrationview.Item{{Entry: task, Parent: parent, LastTouched: base}}
	m.migration.Targets = []string{"Future", "Inbox"}
	m.migration.TargetMetas = map[string]collection.Meta{
		"Future": {Name: "Future", Type: collection.TypeMonthly},
		"Inbox":  {Name: "Inbox", Type: collection.TypeGeneric},
	}
	m.migration.TargetIndex = 1
	m.migration.Focus = migrationview.FocusTargets
	m.updateMigrationViewport()

	view := stripANSI(m.migration.View())
	if !strings.Contains(view, "Migration · last 1 week") {
		t.Fatalf("expected migration header; view=%q", view)
	}
	if !strings.Contains(view, "Finalize agenda") {
		t.Fatalf("expected task message in migration view; view=%q", view)
	}

	m.handleMigrationAfterAction(task.ID, newTestEntry("clone-1", "Future", "Finalize agenda", base))
	m.updateMigrationViewport()
	updated := stripANSI(m.migration.View())
	if !strings.Contains(updated, "Collections") {
		t.Fatalf("expected collections header; view=%q", updated)
	}
}
