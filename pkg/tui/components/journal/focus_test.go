package journal

import (
	"testing"

	"tableflip.dev/bujo/pkg/tui/events"
)

func TestJournalFocusMsgUpdatesFocus(t *testing.T) {
	model := NewModel(nil, nil, nil)
	model.SetID(events.ComponentID("journal"))

	next, _ := model.Update(events.JournalFocusMsg{
		Component: model.ID(),
		Pane:      events.JournalFocusDetail,
	})
	updated, ok := next.(*Model)
	if !ok {
		t.Fatalf("expected *Model, got %T", next)
	}
	if updated.FocusedPane() != FocusDetail {
		t.Fatalf("expected focus detail, got %v", updated.FocusedPane())
	}

	next, _ = updated.Update(events.JournalFocusMsg{
		Component: updated.ID(),
		Pane:      events.JournalFocusNav,
	})
	updated, _ = next.(*Model)
	if updated.FocusedPane() != FocusNav {
		t.Fatalf("expected focus nav, got %v", updated.FocusedPane())
	}

	other, _ := updated.Update(events.JournalFocusMsg{
		Component: events.ComponentID("other"),
		Pane:      events.JournalFocusDetail,
	})
	updated, _ = other.(*Model)
	if updated.FocusedPane() != FocusNav {
		t.Fatalf("expected focus to remain nav")
	}

}
