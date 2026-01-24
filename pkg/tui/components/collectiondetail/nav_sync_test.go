package collectiondetail

import (
	"testing"

	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/tui/events"
)

func TestNavHighlightPreviewDoesNotMoveCursorWhenBlurred(t *testing.T) {
	model := NewModel([]Section{
		{
			ID:    "Inbox",
			Title: "Inbox",
			Bullets: []Bullet{
				{ID: "b1", Label: "First", Bullet: glyph.Task},
			},
		},
		{
			ID:    "Today",
			Title: "Today",
			Bullets: []Bullet{
				{ID: "b2", Label: "Second", Bullet: glyph.Task},
			},
		},
	})
	model.SetSize(40, 6)
	model.cursor = 0
	model.activeSection = 0

	model.handleNavHighlight(events.CollectionHighlightMsg{
		Component: "",
		Collection: events.CollectionRef{
			ID:   "Today",
			Name: "Today",
		},
	})

	if model.activeSection != 1 {
		t.Fatalf("expected active section 1, got %d", model.activeSection)
	}
	if model.cursor != 0 {
		t.Fatalf("expected cursor to remain 0, got %d", model.cursor)
	}
}
