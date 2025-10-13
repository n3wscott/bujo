package store

import (
	"context"
	"testing"
	"time"

	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
)

type testConfig struct {
	path string
}

func (t testConfig) BasePath() string {
	return t.path
}

func TestPersistenceWatchEmitsCollectionChanges(t *testing.T) {
	base := t.TempDir()
	p, err := Load(testConfig{path: base})
	if err != nil {
		t.Fatalf("load persistence: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := p.Watch(ctx)
	if err != nil {
		t.Fatalf("watch: %v", err)
	}

	// Allow watcher goroutine to subscribe to directories before storing.
	time.Sleep(50 * time.Millisecond)

	e := entry.New("Inbox", glyph.Task, "hello world")
	if err := p.Store(e); err != nil {
		t.Fatalf("store entry: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case evt := <-ch:
			if evt.Type == EventCollectionsInvalidated {
				return
			}
			if evt.Type == EventCollectionChanged {
				if evt.Collection != "Inbox" {
					t.Fatalf("expected collection 'Inbox', got %q", evt.Collection)
				}
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for collection change event")
		}
	}
}
