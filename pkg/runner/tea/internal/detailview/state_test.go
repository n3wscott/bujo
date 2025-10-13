package detailview

import (
	"testing"

	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
)

func makeEntries(count int) []*entry.Entry {
	entries := make([]*entry.Entry, count)
	for i := 0; i < count; i++ {
		entries[i] = &entry.Entry{
			ID:      formatID(i),
			Message: "item",
			Bullet:  glyph.Task,
		}
	}
	return entries
}

func formatID(i int) string {
	return string(rune('a' + i))
}

func TestSetSectionsPreservesScrollOffset(t *testing.T) {
	entries := makeEntries(10)
	sections := []Section{{
		CollectionID:   "A",
		CollectionName: "A",
		Entries:        entries,
	}}

	state := NewState()
	state.SetSections(sections)
	state.SetActive("A", entries[0].ID)
	state.Viewport(4)

	for i := 0; i < 6; i++ {
		state.MoveEntry(1)
	}
	state.Viewport(4)

	if state.scrollOffset == 0 {
		t.Fatalf("expected scroll offset to advance after moving, got 0")
	}
	before := state.scrollOffset
	currentID := state.ActiveEntryID()

	state.SetSections(sections)
	state.SetActive("A", currentID)
	state.Viewport(4)

	if state.scrollOffset != before {
		t.Fatalf("expected scroll offset %d after reload, got %d", before, state.scrollOffset)
	}

	state.MoveEntry(-1)
	state.Viewport(4)
	if state.scrollOffset > before {
		t.Fatalf("scroll offset increased after moving up: before %d, after %d", before, state.scrollOffset)
	}
}
