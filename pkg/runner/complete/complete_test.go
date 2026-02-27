package complete

import (
	"context"
	"strings"
	"testing"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/store"
)

type completeTestPersistence struct {
	entries    []*entry.Entry
	storeCalls int
}

func (p *completeTestPersistence) MapAll(context.Context) map[string][]*entry.Entry { return nil }

func (p *completeTestPersistence) ListAll(context.Context) []*entry.Entry { return p.entries }

func (p *completeTestPersistence) List(_ context.Context, collection string) []*entry.Entry {
	list := make([]*entry.Entry, 0, len(p.entries))
	for _, e := range p.entries {
		if e.Collection == collection {
			list = append(list, e)
		}
	}
	return list
}

func (p *completeTestPersistence) Collections(context.Context, string) []string { return nil }

func (p *completeTestPersistence) CollectionsMeta(context.Context, string) []collection.Meta {
	return nil
}

func (p *completeTestPersistence) Store(e *entry.Entry) error {
	p.storeCalls++
	for i, existing := range p.entries {
		if existing.ID == e.ID {
			p.entries[i] = e
			return nil
		}
	}
	p.entries = append(p.entries, e)
	return nil
}

func (p *completeTestPersistence) Delete(*entry.Entry) error { return nil }

func (p *completeTestPersistence) DeleteCollection(context.Context, string) error { return nil }

func (p *completeTestPersistence) EnsureCollection(string) error { return nil }

func (p *completeTestPersistence) EnsureCollectionTyped(string, collection.Type) error { return nil }

func (p *completeTestPersistence) SetCollectionType(string, collection.Type) error { return nil }

func (p *completeTestPersistence) Watch(context.Context) (<-chan store.Event, error) { return nil, nil }

func TestCompleteDoMarksEntryCompleted(t *testing.T) {
	p := &completeTestPersistence{
		entries: []*entry.Entry{{
			ID:         "id-1",
			Collection: "Inbox",
			Bullet:     glyph.Task,
			Message:    "follow up",
		}},
	}

	c := Complete{
		ID:          "id-1",
		Persistence: p,
	}
	if err := c.Do(context.Background()); err != nil {
		t.Fatalf("expected completion to succeed: %v", err)
	}
	if got := p.entries[0].Bullet; got != glyph.Completed {
		t.Fatalf("expected bullet %q, got %q", glyph.Completed, got)
	}
	if p.storeCalls != 1 {
		t.Fatalf("expected one store call, got %d", p.storeCalls)
	}
}

func TestCompleteDoReturnsErrorWhenIDMissing(t *testing.T) {
	p := &completeTestPersistence{
		entries: []*entry.Entry{{
			ID:         "id-1",
			Collection: "Inbox",
			Bullet:     glyph.Task,
			Message:    "follow up",
		}},
	}

	c := Complete{
		ID:          "missing-id",
		Persistence: p,
	}
	err := c.Do(context.Background())
	if err == nil {
		t.Fatal("expected an error when ID is missing")
	}
	if !strings.Contains(err.Error(), "missing-id") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
	if p.storeCalls != 0 {
		t.Fatalf("expected no store calls, got %d", p.storeCalls)
	}
}
