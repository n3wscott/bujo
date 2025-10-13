package detailview

import (
	"fmt"
	"testing"

	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
)

func makeEntries(count int) []*entry.Entry {
	entries := make([]*entry.Entry, count)
	for i := 0; i < count; i++ {
		e := &entry.Entry{
			ID:      formatID(i),
			Message: "item",
			Bullet:  glyph.Task,
		}
		e.EnsureHistorySeed()
		entries[i] = e
	}
	return entries
}

func formatID(i int) string {
	return fmt.Sprintf("%03d", i)
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

func TestRevealCollectionPrefersFullView(t *testing.T) {
	secA := Section{CollectionID: "A", CollectionName: "A", Entries: makeEntries(2)}
	secB := Section{CollectionID: "B", CollectionName: "B", Entries: makeEntries(1)}
	secC := Section{CollectionID: "C", CollectionName: "C", Entries: makeEntries(6)}
	state := NewState()
	state.SetSections([]Section{secA, secB, secC})
	state.SetActive("C", "000")
	state.Viewport(6)

	state.RevealCollection("B", true, 6)
	if state.scrollOffset != 1 {
		t.Fatalf("expected scroll offset 1 to show entire section B, got %d", state.scrollOffset)
	}

	state.RevealCollection("C", true, 6)
	if state.scrollOffset != 7 {
		t.Fatalf("expected scroll offset 7 to pin header for large section, got %d", state.scrollOffset)
	}
}
