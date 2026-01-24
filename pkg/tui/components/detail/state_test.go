package detail

import "testing"

import "tableflip.dev/bujo/pkg/entry"

func TestMoveEntryAcrossSections(t *testing.T) {
	state := NewState()
	state.SetSections([]Section{
		{
			CollectionID:   "A",
			CollectionName: "A",
			Entries: []*entry.Entry{
				{ID: "a1", Message: "first"},
				{ID: "a2", Message: "second"},
			},
		},
		{
			CollectionID:   "B",
			CollectionName: "B",
			Entries: []*entry.Entry{
				{ID: "b1", Message: "third"},
			},
		},
	})
	state.SetCursor(0, 1)
	if !state.MoveEntry(1) {
		t.Fatalf("expected move to succeed")
	}
	if got := state.ActiveEntryID(); got != "b1" {
		t.Fatalf("expected to land on b1, got %q", got)
	}
}

func TestToggleEntryFoldInvalidatesHeights(t *testing.T) {
	state := NewState()
	state.SetSections([]Section{
		{
			CollectionID:   "A",
			CollectionName: "A",
			Entries: []*entry.Entry{
				{ID: "p", Message: "parent"},
				{ID: "c", ParentID: "p", Message: "child"},
			},
		},
	})
	_, _ = state.Viewport(8)
	if len(state.cachedHeights) == 0 || state.cachedHeights[0] == -1 {
		t.Fatalf("expected cached heights to be computed before fold")
	}
	state.ToggleEntryFold("p", true)
	if state.cachedHeights[0] != -1 {
		t.Fatalf("expected cached heights to invalidate after fold")
	}
}
