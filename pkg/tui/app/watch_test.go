package app

import (
	"testing"

	"tableflip.dev/bujo/pkg/collection"
	cachepkg "tableflip.dev/bujo/pkg/tui/cache"
	"tableflip.dev/bujo/pkg/tui/events"
)

func TestCacheListenCmdWrapsChildMsg(t *testing.T) {
	cache := cachepkg.New(events.ComponentID("cache-test"))
	cache.SetCollections([]collection.Meta{
		{Name: "daily/2026-01-23", Type: collection.TypeDaily},
	})

	cmd := cacheListenCmd(cache)
	if cmd == nil {
		t.Fatal("expected cache listen cmd")
	}

	msg := cmd()
	child, ok := msg.(events.ChildMsg)
	if !ok {
		t.Fatalf("expected ChildMsg, got %T", msg)
	}
	if child.From != cache.ComponentID() {
		t.Fatalf("expected from %q, got %q", cache.ComponentID(), child.From)
	}
	if child.Msg == nil {
		t.Fatal("expected child message payload")
	}
}
