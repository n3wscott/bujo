package teaui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/runner/tea/internal/detailview"
	migrationview "tableflip.dev/bujo/pkg/runner/tea/internal/views/migration"
	wizardview "tableflip.dev/bujo/pkg/runner/tea/internal/views/wizard"
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
	m.detailState.SetSections([]detailview.Section{
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
