package add

import (
	"context"
	"testing"
	"time"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/store"
)

type addTestPersistence struct {
	entries []*entry.Entry
}

// MapAll implements store.Persistence.
func (p *addTestPersistence) MapAll(context.Context) map[string][]*entry.Entry { return nil }

// ListAll implements store.Persistence.
func (p *addTestPersistence) ListAll(context.Context) []*entry.Entry { return p.entries }

// List implements store.Persistence.
func (p *addTestPersistence) List(_ context.Context, collection string) []*entry.Entry {
	var list []*entry.Entry
	for _, e := range p.entries {
		if e.Collection == collection {
			list = append(list, e)
		}
	}
	return list
}

// Collections implements store.Persistence.
func (p *addTestPersistence) Collections(context.Context, string) []string { return nil }

// CollectionsMeta implements store.Persistence.
func (p *addTestPersistence) CollectionsMeta(context.Context, string) []collection.Meta { return nil }

// Store implements store.Persistence.
func (p *addTestPersistence) Store(e *entry.Entry) error {
	p.entries = append(p.entries, e)
	return nil
}

// Delete implements store.Persistence.
func (p *addTestPersistence) Delete(*entry.Entry) error { return nil }

// DeleteCollection implements store.Persistence.
func (p *addTestPersistence) DeleteCollection(context.Context, string) error { return nil }

// EnsureCollection implements store.Persistence.
func (p *addTestPersistence) EnsureCollection(string) error { return nil }

// EnsureCollectionTyped implements store.Persistence.
func (p *addTestPersistence) EnsureCollectionTyped(string, collection.Type) error { return nil }

// SetCollectionType implements store.Persistence.
func (p *addTestPersistence) SetCollectionType(string, collection.Type) error { return nil }

// Watch implements store.Persistence.
func (p *addTestPersistence) Watch(context.Context) (<-chan store.Event, error) { return nil, nil }

func TestAddDoPersistsEntryAndSignifier(t *testing.T) {
	p := &addTestPersistence{}
	add := Add{
		Bullet:      glyph.Task,
		Collection:  "today",
		Message:     "hello",
		Priority:    true,
		Persistence: p,
	}
	if err := add.Do(context.Background()); err != nil {
		t.Fatalf("expected add to succeed: %v", err)
	}
	if len(p.entries) != 1 {
		t.Fatalf("expected 1 entry stored, got %d", len(p.entries))
	}
	if p.entries[0].Signifier != glyph.Priority {
		t.Fatalf("expected priority signifier to be set")
	}
	want := time.Now().Format(layoutUS)
	if p.entries[0].Collection != want {
		t.Fatalf("expected collection to be today (%q), got %q", want, p.entries[0].Collection)
	}
}
