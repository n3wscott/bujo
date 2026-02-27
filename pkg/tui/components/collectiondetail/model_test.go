package collectiondetail

import (
	"fmt"
	"strings"
	"testing"

	"github.com/muesli/reflow/ansi"

	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/tui/events"
)

func stripANSIString(s string) string {
	var b strings.Builder
	ansiSeq := false
	for _, r := range s {
		if r == ansi.Marker {
			ansiSeq = true
			continue
		}
		if ansiSeq {
			if ansi.IsTerminator(r) {
				ansiSeq = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func makeBullet(id, label string) Bullet {
	return Bullet{
		ID:     id,
		Label:  label,
		Bullet: glyph.Task,
	}
}

func TestEnsureScrollAccountsForStickyHeaderSpacing(t *testing.T) {
	longLine := "Write an extra long review comment about the storage refactor PR so we can verify wrapping works for nested subtasks that exceed the available width in the detail pane."
	inbox := []Bullet{
		makeBullet("inbox-01", longLine),
		makeBullet("inbox-02", "Email Alex about the demo"),
		makeBullet("inbox-03", "Archive old OKR doc"),
	}

	projects := make([]Bullet, 0, 12)
	for i := 0; i < 12; i++ {
		projects = append(projects, makeBullet(
			fmt.Sprintf("proj-%02d", i),
			fmt.Sprintf("Metrics dashboard polish%02d", i),
		))
	}

	model := NewModel([]Section{
		{ID: "Inbox", Title: "Inbox", Bullets: inbox},
		{ID: "Projects", Title: "Projects", Bullets: projects},
	})
	model.SetSize(60, 9)
	model.Focus()

	if ok := model.focusBulletByID("Projects", "proj-11"); !ok {
		t.Fatalf("expected to focus bullet proj-11")
	}

	target := model.currentLineIndex()
	if target < 0 {
		t.Fatalf("no current line after focusing bullet")
	}
	contentHeight := model.viewportContentHeight()
	if contentHeight <= 0 {
		t.Fatalf("invalid content height %d", contentHeight)
	}
	top := model.scroll
	bottom := top + contentHeight - 1
	if target < top || target > bottom {
		t.Fatalf("target line %d outside viewport [%d,%d] (sticky=%d height=%d)",
			target, top, bottom, model.stickyHeaderHeight(), contentHeight)
	}

	view := model.View()
	plain := stripANSIString(view)
	if !strings.Contains(plain, "Metrics dashboard polish11") {
		t.Fatalf("expected focused bullet to be visible, got:\n%s", plain)
	}
}

func TestPlaceholderSectionStaysVisibleWhenCursorEmpty(t *testing.T) {
	bullets := make([]Bullet, 0, 12)
	for i := 0; i < 12; i++ {
		bullets = append(bullets, makeBullet(
			fmt.Sprintf("task-%02d", i),
			fmt.Sprintf("Task %02d", i),
		))
	}

	model := NewModel([]Section{
		{ID: "Inbox", Title: "Inbox", Bullets: bullets},
		{ID: "Today", Title: "Today", Placeholder: true},
	})
	model.SetSize(40, 6)
	model.Focus()

	model.FocusCollection("Today")
	model.cursor = -1
	model.ensureScroll()

	view := stripANSIString(model.View())
	if !strings.Contains(view, "Today") {
		t.Fatalf("expected Today header visible, got:\n%s", view)
	}
	if !strings.Contains(view, "collection not yet created") {
		t.Fatalf("expected placeholder message, got:\n%s", view)
	}
}

func TestSelectionRetainedAfterReorderAndPlaceholderInsert(t *testing.T) {
	model := NewModel([]Section{
		{ID: "A", Title: "A", Bullets: []Bullet{makeBullet("a1", "A1")}},
		{ID: "B", Title: "B", Bullets: []Bullet{makeBullet("b1", "B1")}},
	})
	model.SetSize(40, 8)
	model.Focus()
	if ok := model.focusBulletByID("B", "b1"); !ok {
		t.Fatalf("expected to focus bullet b1")
	}

	if !model.reorderSections([]string{"B", "A"}) {
		t.Fatalf("expected reorder to change section order")
	}
	model.rebuildLookup()
	model.refreshFromSections(false)
	section, bullet, ok := model.CurrentSelection()
	if !ok || section.ID != "B" || bullet.ID != "b1" {
		t.Fatalf("expected selection to remain on b1, got section=%q bullet=%q", section.ID, bullet.ID)
	}

	model.ensurePlaceholderSection(events.CollectionRef{ID: "C", Name: "C"})
	section, bullet, ok = model.CurrentSelection()
	if !ok || section.ID != "B" || bullet.ID != "b1" {
		t.Fatalf("expected selection to remain on b1 after placeholder insert, got section=%q bullet=%q", section.ID, bullet.ID)
	}
}

func TestHighlightAndSelectPlaceholderBehavior(t *testing.T) {
	model := NewModel([]Section{
		{ID: "Inbox", Title: "Inbox", Bullets: []Bullet{makeBullet("a1", "A1")}},
	})
	model.SetSize(40, 8)
	model.Focus()

	_, _ = model.Update(events.CollectionHighlightMsg{
		Component:  "nav",
		Collection: events.CollectionRef{ID: "Day/One", Name: "One"},
		RowKind:    "day",
	})
	if idx := model.sectionIndexForCollection(events.CollectionRef{ID: "Day/One", Name: "One"}); idx < 0 {
		t.Fatalf("expected placeholder section to be created for day highlight")
	}

	before := len(model.sections)
	_, _ = model.Update(events.CollectionHighlightMsg{
		Component:  "nav",
		Collection: events.CollectionRef{ID: "Missing", Name: "Missing"},
		RowKind:    "generic",
	})
	if len(model.sections) != before {
		t.Fatalf("expected non-day highlight to avoid creating placeholder section")
	}

	_, _ = model.Update(events.CollectionSelectMsg{
		Component:  "nav",
		Collection: events.CollectionRef{ID: "Select/Me", Name: "Select"},
		RowKind:    "day",
		Exists:     false,
	})
	if idx := model.sectionIndexForCollection(events.CollectionRef{ID: "Select/Me", Name: "Select"}); idx < 0 {
		t.Fatalf("expected placeholder section to be created for missing select")
	}
}

func TestViewRendersLabelsBeneathBullet(t *testing.T) {
	model := NewModel([]Section{
		{
			ID:    "Inbox",
			Title: "Inbox",
			Bullets: []Bullet{{
				ID:     "task-1",
				Label:  "Follow up",
				Bullet: glyph.Task,
				Labels: []string{"area:api", "owner:codex"},
			}},
		},
	})
	model.SetSize(60, 8)
	model.Focus()

	view := stripANSIString(model.View())
	if !strings.Contains(view, "area:api, owner:codex") {
		t.Fatalf("expected labels line in view, got:\n%s", view)
	}
	if strings.Contains(view, "labels:") {
		t.Fatalf("expected labels prefix to be removed, got:\n%s", view)
	}
}
