package collectiondetail2

import (
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

func makeBullet(id string) Bullet {
	return Bullet{
		ID:     id,
		Label:  id,
		Bullet: glyph.Task,
	}
}

func TestCursorClampsWithinSection(t *testing.T) {
	model := NewModel([]Section{
		{ID: "A", Title: "A", Bullets: []Bullet{makeBullet("a1"), makeBullet("a2")}},
		{ID: "B", Title: "B", Bullets: []Bullet{makeBullet("b1"), makeBullet("b2")}},
	})
	model.SetSize(40, 8)
	model.Focus()
	model.SetMode(ModeFocused)
	model.FocusCollection("A")

	model.moveCursor(1)
	model.moveCursor(1)

	section, bullet, ok := model.CurrentSelection()
	if !ok {
		t.Fatal("expected selection after moving")
	}
	if section.ID != "A" || bullet.ID != "a2" {
		t.Fatalf("expected to clamp at A/a2, got %q/%q", section.ID, bullet.ID)
	}
}

func TestCursorClampUpDoesNotCrossSection(t *testing.T) {
	model := NewModel([]Section{
		{ID: "A", Title: "A", Bullets: []Bullet{makeBullet("a1"), makeBullet("a2")}},
		{ID: "B", Title: "B", Bullets: []Bullet{makeBullet("b1"), makeBullet("b2")}},
	})
	model.SetSize(40, 8)
	model.Focus()
	model.SetMode(ModeFocused)
	model.FocusCollection("B")

	model.cursor = 0
	model.moveCursor(-1)

	section, bullet, ok := model.CurrentSelection()
	if !ok {
		t.Fatal("expected selection after moving up")
	}
	if section.ID != "B" || bullet.ID != "b1" {
		t.Fatalf("expected to clamp at B/b1, got %q/%q", section.ID, bullet.ID)
	}
}

func TestCursorCrossesSectionInContinuousMode(t *testing.T) {
	model := NewModel([]Section{
		{ID: "A", Title: "A", Bullets: []Bullet{makeBullet("a1"), makeBullet("a2")}},
		{ID: "B", Title: "B", Bullets: []Bullet{makeBullet("b1"), makeBullet("b2")}},
	})
	model.SetSize(40, 8)
	model.Focus()
	model.FocusCollection("A")

	model.moveCursor(1)
	model.moveCursor(1)

	section, bullet, ok := model.CurrentSelection()
	if !ok {
		t.Fatal("expected selection after moving")
	}
	if section.ID != "B" || bullet.ID != "b1" {
		t.Fatalf("expected selection to continue into B/b1, got %q/%q", section.ID, bullet.ID)
	}
}

func TestCursorSkipsEmptySectionInContinuousMode(t *testing.T) {
	model := NewModel([]Section{
		{ID: "A", Title: "A", Bullets: nil},
		{ID: "B", Title: "B", Bullets: []Bullet{makeBullet("b1")}},
	})
	model.SetSize(40, 8)
	model.Focus()
	model.FocusCollection("A")

	model.moveCursor(1)

	section, bullet, ok := model.CurrentSelection()
	if !ok {
		t.Fatal("expected selection after moving")
	}
	if section.ID != "B" || bullet.ID != "b1" {
		t.Fatalf("expected selection to move to B/b1, got %q/%q", section.ID, bullet.ID)
	}
}

func TestNewBulletAppendsWithinSection(t *testing.T) {
	model := NewModel([]Section{
		{ID: "A", Title: "A", Bullets: []Bullet{makeBullet("a1")}},
		{ID: "B", Title: "B", Bullets: []Bullet{makeBullet("b1")}},
	})
	model.SetSize(40, 8)
	model.Focus()
	model.FocusCollection("B")

	_, cmd := model.Update(events.BulletChangeMsg{
		Action:    events.ChangeCreate,
		Component: "detail",
		Collection: events.CollectionViewRef{
			ID:    "A",
			Title: "A",
		},
		Bullet: events.BulletRef{
			ID:     "a2",
			Label:  "a2",
			Bullet: glyph.Task,
		},
	})
	if cmd != nil {
		_ = cmd()
	}

	if got := len(model.sections); got != 2 {
		t.Fatalf("expected 2 sections, got %d", got)
	}
	if model.sections[0].ID != "A" || model.sections[1].ID != "B" {
		t.Fatalf("expected section order A, B, got %q, %q", model.sections[0].ID, model.sections[1].ID)
	}
	if got := len(model.sections[0].Bullets); got != 2 {
		t.Fatalf("expected 2 bullets in section A, got %d", got)
	}
	if model.sections[0].Bullets[0].ID != "a1" || model.sections[0].Bullets[1].ID != "a2" {
		t.Fatalf("expected section A bullets to stay in insertion order, got %q then %q", model.sections[0].Bullets[0].ID, model.sections[0].Bullets[1].ID)
	}

	section, bullet, ok := model.CurrentSelection()
	if !ok || section.ID != "A" || bullet.ID != "a2" {
		t.Fatalf("expected selection to move to new bullet A/a2, got section=%q bullet=%q ok=%v", section.ID, bullet.ID, ok)
	}
}

func TestModeSwitchingRendersFocusedSection(t *testing.T) {
	model := NewModel([]Section{
		{ID: "A", Title: "A", Bullets: []Bullet{makeBullet("a1")}},
		{ID: "B", Title: "B", Bullets: []Bullet{makeBullet("b1")}},
	})
	model.SetSize(40, 8)
	model.Focus()
	model.FocusCollection("B")

	if !model.SetMode(ModeFocused) {
		t.Fatalf("expected mode switch to focused to succeed")
	}
	for _, info := range model.lines {
		if info.section != model.activeSection {
			t.Fatalf("expected focused mode to render only active section, found section %d (active %d)", info.section, model.activeSection)
		}
	}

	if !model.SetMode(ModeContinuous) {
		t.Fatalf("expected mode switch back to continuous to succeed")
	}
	foundA := false
	foundB := false
	for _, info := range model.lines {
		if info.kind != lineHeader {
			continue
		}
		if info.section == 0 {
			foundA = true
		}
		if info.section == 1 {
			foundB = true
		}
	}
	if !foundA || !foundB {
		t.Fatalf("expected continuous mode to render both section headers, got A=%v B=%v", foundA, foundB)
	}
}

func TestNavHighlightPreviewWhenUnfocused(t *testing.T) {
	model := NewModel([]Section{
		{ID: "A", Title: "A", Bullets: []Bullet{makeBullet("a1")}},
		{ID: "B", Title: "B", Bullets: []Bullet{makeBullet("b1")}},
	})
	model.SetSize(40, 8)

	_, cmd := model.Update(events.CollectionHighlightMsg{
		Component:  "nav",
		Collection: events.CollectionRef{ID: "B", Name: "B"},
	})
	if cmd != nil {
		t.Fatalf("expected no highlight cmd when unfocused")
	}
	if model.activeSection != 1 {
		t.Fatalf("expected active section to preview B, got %d", model.activeSection)
	}
	if model.lastHighlight != "" {
		t.Fatalf("expected no highlight state to be recorded when unfocused")
	}
}

func TestNavHighlightFocusedEmitsHighlight(t *testing.T) {
	model := NewModel([]Section{
		{ID: "A", Title: "A", Bullets: []Bullet{makeBullet("a1")}},
		{ID: "B", Title: "B", Bullets: []Bullet{makeBullet("b1")}},
	})
	model.SetSize(40, 8)
	model.Focus()

	_, cmd := model.Update(events.CollectionHighlightMsg{
		Component:  "nav",
		Collection: events.CollectionRef{ID: "B", Name: "B"},
	})
	if cmd == nil {
		t.Fatalf("expected highlight cmd when focused")
	}
	msg := cmd()
	highlight, ok := msg.(events.BulletHighlightMsg)
	if !ok {
		t.Fatalf("expected BulletHighlightMsg, got %T", msg)
	}
	if highlight.Collection.ID != "B" {
		t.Fatalf("expected highlight for section B, got %q", highlight.Collection.ID)
	}
}

func TestReorderSectionsKeepsSelection(t *testing.T) {
	model := NewModel([]Section{
		{ID: "A", Title: "A", Bullets: []Bullet{makeBullet("a1")}},
		{ID: "B", Title: "B", Bullets: []Bullet{makeBullet("b1")}},
	})
	model.SetSize(40, 8)
	model.Focus()
	model.FocusCollection("B")

	_, _ = model.Update(events.CollectionOrderMsg{
		Component: "nav",
		Order:     []string{"B", "A"},
	})

	if model.sections[0].ID != "B" || model.sections[1].ID != "A" {
		t.Fatalf("expected section order B, A after reorder, got %q, %q", model.sections[0].ID, model.sections[1].ID)
	}
	section, bullet, ok := model.CurrentSelection()
	if !ok || section.ID != "B" || bullet.ID != "b1" {
		t.Fatalf("expected selection to remain on B/b1, got section=%q bullet=%q ok=%v", section.ID, bullet.ID, ok)
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

func TestCompletedBulletsDimLabelsStyle(t *testing.T) {
	model := NewModel(nil)

	_, _, labelsStyle := model.bulletStyles(Bullet{Bullet: glyph.Task})
	normalRendered := labelsStyle.Render("owner:codex")
	if !strings.Contains(normalRendered, "38;5;214m") {
		t.Fatalf("expected active labels to render mustard, got: %q", normalRendered)
	}

	_, _, labelsStyle = model.bulletStyles(Bullet{Bullet: glyph.Completed})
	completedRendered := labelsStyle.Render("owner:codex")
	if !strings.Contains(completedRendered, "38;5;241m") {
		t.Fatalf("expected completed labels to render dim gray, got: %q", completedRendered)
	}
}
