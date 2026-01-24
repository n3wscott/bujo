package journal

import (
	"testing"

	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/components/collectionnav"
	"tableflip.dev/bujo/pkg/tui/events"
)

func TestHandleSelectionEventBulletSelect(t *testing.T) {
	nav := collectionnav.NewModel(nil)
	detail := collectiondetail.NewModel(nil)
	model := NewModel(nav, detail, nil)

	cmds := model.handleSelectionEvent(events.BulletSelectMsg{
		Component: detail.ID(),
		Collection: events.CollectionViewRef{
			ID:    "daily/2026-01-23",
			Title: "2026-01-23",
		},
		Bullet: events.BulletRef{ID: "bullet-1", Label: "Test"},
	})
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	msg := cmds[0]()
	req, ok := msg.(events.BulletDetailRequestMsg)
	if !ok {
		t.Fatalf("expected BulletDetailRequestMsg, got %T", msg)
	}
	if req.Collection.ID != "daily/2026-01-23" {
		t.Fatalf("expected collection ID to match, got %q", req.Collection.ID)
	}
}

func TestHandleSelectionEventCollectionSelect(t *testing.T) {
	nav := collectionnav.NewModel(nil)
	detail := collectiondetail.NewModel(nil)
	model := NewModel(nav, detail, nil)

	cmds := model.handleSelectionEvent(events.CollectionSelectMsg{
		Component: nav.ID(),
		Collection: events.CollectionRef{
			ID:   "daily/2026-01-23",
			Name: "2026-01-23",
		},
	})
	if len(cmds) == 0 {
		t.Fatal("expected focus command for collection select")
	}
	if cmds[0] == nil {
		t.Fatal("expected non-nil focus command")
	}
}
