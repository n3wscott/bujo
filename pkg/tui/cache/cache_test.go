package cache

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/store"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/events"
)

type fakePersistence struct {
	entries map[string][]*entry.Entry
	metas   map[string]collection.Meta
	nextID  int
}

// newFakePersistence seeds an in-memory persistence store for cache tests.
func newFakePersistence() *fakePersistence {
	return &fakePersistence{
		entries: make(map[string][]*entry.Entry),
		metas:   make(map[string]collection.Meta),
		nextID:  1,
	}
}

func (f *fakePersistence) MapAll(_ context.Context) map[string][]*entry.Entry {
	out := make(map[string][]*entry.Entry, len(f.entries))
	for k, v := range f.entries {
		clone := make([]*entry.Entry, len(v))
		copy(clone, v)
		out[k] = clone
	}
	return out
}

func (f *fakePersistence) ListAll(_ context.Context) []*entry.Entry {
	var all []*entry.Entry
	for _, list := range f.entries {
		all = append(all, list...)
	}
	return all
}

func (f *fakePersistence) List(_ context.Context, collection string) []*entry.Entry {
	list := f.entries[collection]
	clone := make([]*entry.Entry, len(list))
	copy(clone, list)
	return clone
}

func (f *fakePersistence) Collections(_ context.Context, prefix string) []string {
	var names []string
	for name := range f.metas {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func (f *fakePersistence) CollectionsMeta(_ context.Context, prefix string) []collection.Meta {
	var metas []collection.Meta
	for name, meta := range f.metas {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			metas = append(metas, meta)
		}
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].Name < metas[j].Name })
	return metas
}

func (f *fakePersistence) Store(e *entry.Entry) error {
	if e == nil {
		return nil
	}
	if e.ID == "" {
		e.ID = f.newID()
	}
	list := f.entries[e.Collection]
	for i := range list {
		if list[i].ID == e.ID {
			list[i] = e
			f.entries[e.Collection] = list
			return nil
		}
	}
	f.entries[e.Collection] = append(list, e)
	return nil
}

func (f *fakePersistence) Delete(e *entry.Entry) error {
	if e == nil {
		return nil
	}
	list := f.entries[e.Collection]
	for i := range list {
		if list[i].ID == e.ID {
			list = append(list[:i], list[i+1:]...)
			f.entries[e.Collection] = list
			return nil
		}
	}
	return nil
}

func (f *fakePersistence) DeleteCollection(_ context.Context, name string) error {
	delete(f.entries, name)
	delete(f.metas, name)
	return nil
}

func (f *fakePersistence) EnsureCollection(name string) error {
	if _, ok := f.metas[name]; !ok {
		f.metas[name] = collection.Meta{Name: name, Type: collection.TypeGeneric}
	}
	return nil
}

func (f *fakePersistence) EnsureCollectionTyped(name string, typ collection.Type) error {
	f.metas[name] = collection.Meta{Name: name, Type: typ}
	return nil
}

func (f *fakePersistence) SetCollectionType(name string, typ collection.Type) error {
	f.metas[name] = collection.Meta{Name: name, Type: typ}
	return nil
}

func (f *fakePersistence) Watch(_ context.Context) (<-chan store.Event, error) {
	ch := make(chan store.Event)
	return ch, nil
}

func (f *fakePersistence) newID() string {
	id := f.nextID
	f.nextID++
	return "id-" + strconv.Itoa(id)
}

func TestApplySnapshotEmitsCollectionChanges(t *testing.T) {
	cache := New("cache-test")
	initial := Snapshot{
		Metas: []collection.Meta{
			{Name: "Alpha", Type: collection.TypeGeneric},
		},
		Sections: []collectiondetail.Section{{ID: "Alpha", Title: "Alpha", Subtitle: "generic"}},
	}
	cache.ApplySnapshot(initial)
	drainEvents(cache.Events())

	next := Snapshot{
		Metas: []collection.Meta{
			{Name: "Alpha", Type: collection.TypeTracking},
			{Name: "Beta", Type: collection.TypeGeneric},
		},
		Sections: []collectiondetail.Section{{ID: "Alpha", Title: "Alpha", Subtitle: "tracking"}},
	}
	cache.ApplySnapshot(next)

	var creates, updates int
	for _, msg := range drainEvents(cache.Events()) {
		change, ok := msg.(events.CollectionChangeMsg)
		if !ok {
			continue
		}
		switch change.Action {
		case events.ChangeCreate:
			creates++
		case events.ChangeUpdate:
			updates++
		}
	}
	if creates != 1 {
		t.Fatalf("expected 1 collection create, got %d", creates)
	}
	if updates != 1 {
		t.Fatalf("expected 1 collection update, got %d", updates)
	}
}

func TestApplySnapshotEmitsBulletUpdates(t *testing.T) {
	cache := New("cache-test")
	initial := Snapshot{
		Metas:    []collection.Meta{{Name: "Alpha", Type: collection.TypeGeneric}},
		Sections: []collectiondetail.Section{{ID: "Alpha", Bullets: []collectiondetail.Bullet{{ID: "1", Label: "old", Bullet: glyph.Task}}}},
	}
	cache.ApplySnapshot(initial)
	drainEvents(cache.Events())

	next := Snapshot{
		Metas:    []collection.Meta{{Name: "Alpha", Type: collection.TypeGeneric}},
		Sections: []collectiondetail.Section{{ID: "Alpha", Bullets: []collectiondetail.Bullet{{ID: "1", Label: "new", Bullet: glyph.Task}}}},
	}
	cache.ApplySnapshot(next)

	var updates int
	for _, msg := range drainEvents(cache.Events()) {
		change, ok := msg.(events.BulletChangeMsg)
		if !ok {
			continue
		}
		if change.Action == events.ChangeUpdate {
			updates++
		}
	}
	if updates != 1 {
		t.Fatalf("expected 1 bullet update, got %d", updates)
	}
}

func TestApplySnapshotEmitsBulletUpdateForLabelChanges(t *testing.T) {
	cache := New("cache-test")
	initial := Snapshot{
		Metas: []collection.Meta{{Name: "Alpha", Type: collection.TypeGeneric}},
		Sections: []collectiondetail.Section{{
			ID:      "Alpha",
			Bullets: []collectiondetail.Bullet{{ID: "1", Label: "task", Labels: []string{"owner:codex"}, Bullet: glyph.Task}},
		}},
	}
	cache.ApplySnapshot(initial)
	drainEvents(cache.Events())

	next := Snapshot{
		Metas: []collection.Meta{{Name: "Alpha", Type: collection.TypeGeneric}},
		Sections: []collectiondetail.Section{{
			ID:      "Alpha",
			Bullets: []collectiondetail.Bullet{{ID: "1", Label: "task", Labels: []string{"owner:snichols"}, Bullet: glyph.Task}},
		}},
	}
	cache.ApplySnapshot(next)

	var updates int
	for _, msg := range drainEvents(cache.Events()) {
		change, ok := msg.(events.BulletChangeMsg)
		if !ok {
			continue
		}
		if change.Action == events.ChangeUpdate {
			updates++
		}
	}
	if updates != 1 {
		t.Fatalf("expected 1 bullet update for label change, got %d", updates)
	}
}

func TestApplySnapshotEmitsBulletCreateAndDelete(t *testing.T) {
	cache := New("cache-test")
	initial := Snapshot{
		Metas:    []collection.Meta{{Name: "Alpha", Type: collection.TypeGeneric}},
		Sections: []collectiondetail.Section{{ID: "Alpha", Bullets: []collectiondetail.Bullet{{ID: "1", Label: "old", Bullet: glyph.Task}}}},
	}
	cache.ApplySnapshot(initial)
	drainEvents(cache.Events())

	next := Snapshot{
		Metas:    []collection.Meta{{Name: "Alpha", Type: collection.TypeGeneric}},
		Sections: []collectiondetail.Section{{ID: "Alpha", Bullets: []collectiondetail.Bullet{{ID: "2", Label: "new", Bullet: glyph.Task}}}},
	}
	cache.ApplySnapshot(next)

	var creates, deletes int
	for _, msg := range drainEvents(cache.Events()) {
		change, ok := msg.(events.BulletChangeMsg)
		if !ok {
			continue
		}
		switch change.Action {
		case events.ChangeCreate:
			creates++
		case events.ChangeDelete:
			deletes++
		}
	}
	if creates != 1 {
		t.Fatalf("expected 1 bullet create, got %d", creates)
	}
	if deletes != 1 {
		t.Fatalf("expected 1 bullet delete, got %d", deletes)
	}
}

func TestCreateCollectionEmitsChangeAndCreatesSection(t *testing.T) {
	cache := New("cache-test")
	cache.CreateCollection(collection.Meta{Name: "Alpha", Type: collection.TypeDaily})

	if _, ok := cache.SectionSnapshot("Alpha"); !ok {
		t.Fatalf("expected section snapshot for Alpha")
	}

	var creates int
	for _, msg := range drainEvents(cache.Events()) {
		change, ok := msg.(events.CollectionChangeMsg)
		if ok && change.Action == events.ChangeCreate {
			creates++
		}
	}
	if creates != 1 {
		t.Fatalf("expected 1 collection create, got %d", creates)
	}
}

func TestSyncCollectionBuildsSection(t *testing.T) {
	fake := newFakePersistence()
	fake.metas["Alpha"] = collection.Meta{Name: "Alpha", Type: collection.TypeGeneric}
	fake.entries["Alpha"] = []*entry.Entry{newEntry("entry-1", "", time.Now())}

	svc := &app.Service{Persistence: fake}
	cache := NewWithOptions(Options{Component: "cache-test", Service: svc})
	if err := cache.SyncCollection(context.Background(), "Alpha"); err != nil {
		t.Fatalf("expected sync to succeed, got %v", err)
	}

	section, ok := cache.SectionSnapshot("Alpha")
	if !ok {
		t.Fatalf("expected section snapshot for Alpha")
	}
	if len(section.Bullets) != 1 {
		t.Fatalf("expected 1 bullet, got %d", len(section.Bullets))
	}

	var creates int
	for _, msg := range drainEvents(cache.Events()) {
		change, ok := msg.(events.BulletChangeMsg)
		if ok && change.Action == events.ChangeCreate {
			creates++
		}
	}
	if creates != 1 {
		t.Fatalf("expected 1 bullet create event, got %d", creates)
	}
}

func TestCreateBulletPersistedValidatesInputs(t *testing.T) {
	cache := New("cache-test")

	err := cache.createBulletPersisted(context.Background(), &app.Service{}, "", collectiondetail.Bullet{Label: "ok"}, nil)
	if err == nil {
		t.Fatalf("expected error for empty collection id")
	}

	err = cache.createBulletPersisted(context.Background(), &app.Service{}, "Alpha", collectiondetail.Bullet{}, nil)
	if err == nil {
		t.Fatalf("expected error for empty bullet label")
	}
}

func TestCreateBulletPersistedFailsOnMissingParent(t *testing.T) {
	fake := newFakePersistence()
	fake.metas["Alpha"] = collection.Meta{Name: "Alpha", Type: collection.TypeGeneric}
	svc := &app.Service{Persistence: fake}
	cache := NewWithOptions(Options{Component: "cache-test", Service: svc})

	err := cache.createBulletPersisted(context.Background(), svc, "Alpha", collectiondetail.Bullet{Label: "child", Bullet: glyph.Task}, map[string]string{parentMetaKey: "missing"})
	if err == nil {
		t.Fatalf("expected error for missing parent")
	}
}

// drainEvents collects any pending cache events without blocking.
func drainEvents(ch <-chan tea.Msg) []tea.Msg {
	var msgs []tea.Msg
	for {
		select {
		case msg := <-ch:
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}
}
