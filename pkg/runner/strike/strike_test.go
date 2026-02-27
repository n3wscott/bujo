package strike

import (
	"context"
	"strings"
	"testing"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/store"
)

type strikeTestPersistence struct {
	entries    []*entry.Entry
	storeCalls int
}

func (p *strikeTestPersistence) MapAll(context.Context) map[string][]*entry.Entry { return nil }

func (p *strikeTestPersistence) ListAll(context.Context) []*entry.Entry { return p.entries }

func (p *strikeTestPersistence) List(_ context.Context, collection string) []*entry.Entry {
	list := make([]*entry.Entry, 0, len(p.entries))
	for _, e := range p.entries {
		if e.Collection == collection {
			list = append(list, e)
		}
	}
	return list
}

func (p *strikeTestPersistence) Collections(context.Context, string) []string { return nil }

func (p *strikeTestPersistence) CollectionsMeta(context.Context, string) []collection.Meta {
	return nil
}

func (p *strikeTestPersistence) Store(e *entry.Entry) error {
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

func (p *strikeTestPersistence) Delete(*entry.Entry) error { return nil }

func (p *strikeTestPersistence) DeleteCollection(context.Context, string) error { return nil }

func (p *strikeTestPersistence) EnsureCollection(string) error { return nil }

func (p *strikeTestPersistence) EnsureCollectionTyped(string, collection.Type) error { return nil }

func (p *strikeTestPersistence) SetCollectionType(string, collection.Type) error { return nil }

func (p *strikeTestPersistence) Watch(context.Context) (<-chan store.Event, error) { return nil, nil }

func TestStrikeDoMarksEntryIrrelevant(t *testing.T) {
	p := &strikeTestPersistence{
		entries: []*entry.Entry{{
			ID:         "id-1",
			Collection: "Inbox",
			Bullet:     glyph.Task,
			Signifier:  glyph.Priority,
			Message:    "follow up",
		}},
	}

	s := Strike{
		ID:          "id-1",
		Persistence: p,
	}
	if err := s.Do(context.Background()); err != nil {
		t.Fatalf("expected strike to succeed: %v", err)
	}
	if got := p.entries[0].Bullet; got != glyph.Irrelevant {
		t.Fatalf("expected bullet %q, got %q", glyph.Irrelevant, got)
	}
	if got := p.entries[0].Signifier; got != glyph.None {
		t.Fatalf("expected signifier %q, got %q", glyph.None, got)
	}
	if p.storeCalls != 1 {
		t.Fatalf("expected one store call, got %d", p.storeCalls)
	}
}

func TestStrikeDoReturnsErrorWhenIDMissing(t *testing.T) {
	p := &strikeTestPersistence{
		entries: []*entry.Entry{{
			ID:         "id-1",
			Collection: "Inbox",
			Bullet:     glyph.Task,
			Message:    "follow up",
		}},
	}

	s := Strike{
		ID:          "missing-id",
		Persistence: p,
	}
	err := s.Do(context.Background())
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
