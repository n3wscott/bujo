package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/store"
)

type memoryPersistence struct {
	mu          sync.Mutex
	counter     int
	collections map[string]map[string]*entry.Entry
}

func newMemoryPersistence(entries ...*entry.Entry) *memoryPersistence {
	mp := &memoryPersistence{collections: make(map[string]map[string]*entry.Entry)}
	for _, e := range entries {
		if e == nil {
			continue
		}
		if e.ID == "" {
			e.ID = mp.newID()
		}
		if mp.collections[e.Collection] == nil {
			mp.collections[e.Collection] = make(map[string]*entry.Entry)
		}
		cp := cloneEntry(e)
		mp.collections[e.Collection][cp.ID] = cp
	}
	return mp
}

func (m *memoryPersistence) newID() string {
	m.counter++
	return fmt.Sprintf("id-%d", m.counter)
}

func (m *memoryPersistence) MapAll(_ context.Context) map[string][]*entry.Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string][]*entry.Entry, len(m.collections))
	for col, items := range m.collections {
		for _, e := range items {
			out[col] = append(out[col], cloneEntry(e))
		}
	}
	return out
}

func (m *memoryPersistence) ListAll(_ context.Context) []*entry.Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*entry.Entry
	for _, items := range m.collections {
		for _, e := range items {
			out = append(out, cloneEntry(e))
		}
	}
	return out
}

func (m *memoryPersistence) List(_ context.Context, collection string) []*entry.Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := m.collections[collection]
	out := make([]*entry.Entry, 0, len(items))
	for _, e := range items {
		out = append(out, cloneEntry(e))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (m *memoryPersistence) Collections(_ context.Context, prefix string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cols := make([]string, 0, len(m.collections))
	for col := range m.collections {
		if prefix == "" || strings.HasPrefix(col, prefix) {
			cols = append(cols, col)
		}
	}
	sort.Strings(cols)
	return cols
}

func (m *memoryPersistence) Store(e *entry.Entry) error {
	if e == nil {
		return errors.New("nil entry")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if e.Collection == "" {
		return errors.New("missing collection")
	}
	if e.ID == "" {
		e.ID = m.newID()
	}
	if m.collections[e.Collection] == nil {
		m.collections[e.Collection] = make(map[string]*entry.Entry)
	}
	m.collections[e.Collection][e.ID] = cloneEntry(e)
	return nil
}

func (m *memoryPersistence) Delete(e *entry.Entry) error {
	if e == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	items := m.collections[e.Collection]
	if items == nil {
		return nil
	}
	delete(items, e.ID)
	return nil
}

func (m *memoryPersistence) Watch(context.Context) (<-chan store.Event, error) {
	return nil, nil
}

func (m *memoryPersistence) EnsureCollection(collection string) error {
	if strings.TrimSpace(collection) == "" {
		return errors.New("collection required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.collections[collection] == nil {
		m.collections[collection] = make(map[string]*entry.Entry)
	}
	return nil
}

func cloneEntry(e *entry.Entry) *entry.Entry {
	if e == nil {
		return nil
	}
	cp := &entry.Entry{
		ID:         e.ID,
		Bullet:     e.Bullet,
		Schema:     e.Schema,
		Created:    e.Created,
		Collection: e.Collection,
		Signifier:  e.Signifier,
		Message:    e.Message,
		ParentID:   e.ParentID,
		Immutable:  e.Immutable,
	}
	if e.On != nil {
		on := *e.On
		cp.On = &on
	}
	if len(e.History) > 0 {
		cp.History = append([]entry.HistoryRecord(nil), e.History...)
	}
	return cp
}

func TestSetParentPreventsCycles(t *testing.T) {
	parent := &entry.Entry{ID: "p", Collection: "Inbox", Message: "Parent"}
	child := &entry.Entry{ID: "c", Collection: "Inbox", Message: "Child", ParentID: ""}
	mp := newMemoryPersistence(parent, child)
	svc := &Service{Persistence: mp}
	ctx := context.Background()

	if _, err := svc.SetParent(ctx, "c", "p"); err != nil {
		t.Fatalf("set parent: %v", err)
	}

	if _, err := svc.SetParent(ctx, "p", "c"); err == nil {
		t.Fatal("expected cycle prevention error")
	}
}

func TestMoveMovesSubtree(t *testing.T) {
	parent := &entry.Entry{ID: "p", Collection: "Today", Message: "Parent"}
	child := &entry.Entry{ID: "c", Collection: "Today", Message: "Child", ParentID: "p"}
	mp := newMemoryPersistence(parent, child)
	svc := &Service{Persistence: mp}
	ctx := context.Background()

	clone, err := svc.Move(ctx, "p", "Future")
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if clone == nil {
		t.Fatal("expected clone entry")
	}
	if clone.Collection != "Future" {
		t.Fatalf("expected clone in Future, got %s", clone.Collection)
	}
	if clone.ParentID != "" {
		t.Fatalf("expected clone parent empty, got %q", clone.ParentID)
	}
	if glyph := clone.Bullet; glyph != parent.Bullet {
		t.Fatalf("clone bullet mismatch: %v", glyph)
	}

	future := mp.List(ctx, "Future")
	if len(future) != 2 {
		t.Fatalf("expected 2 entries in Future, got %d", len(future))
	}

	var childClone *entry.Entry
	for _, e := range future {
		if e.ID == clone.ID {
			continue
		}
		if e.Message == "Child" {
			childClone = e
		}
	}
	if childClone == nil {
		t.Fatal("child clone not found")
	}
	if childClone.ParentID != clone.ID {
		t.Fatalf("expected child parent %q, got %q", clone.ID, childClone.ParentID)
	}

	originals := mp.List(ctx, "Today")
	for _, e := range originals {
		if e.ID == "c" && e.Bullet != glyph.MovedFuture {
			t.Fatalf("expected original child bullet to indicate move, got %v", e.Bullet)
		}
	}
}

func TestReportFiltersByWindow(t *testing.T) {
	base := time.Date(2025, 10, 14, 12, 0, 0, 0, time.UTC)
	inWindow := newCompletedEntry("in", "Today", "Done today", base.Add(-48*time.Hour))
	outWindow := newCompletedEntry("out", "Today", "Old task", base.Add(-15*24*time.Hour))
	otherCollection := newCompletedEntry("other", "Work", "Project", base.Add(-24*time.Hour))

	mp := newMemoryPersistence(inWindow, outWindow, otherCollection)
	svc := &Service{Persistence: mp}

	res, err := svc.Report(context.Background(), base.Add(-7*24*time.Hour), base)
	if err != nil {
		t.Fatalf("report: %v", err)
	}

	if res.Total != 2 {
		t.Fatalf("expected 2 entries, got %d", res.Total)
	}
	if len(res.Sections) != 2 {
		t.Fatalf("expected two sections, got %d", len(res.Sections))
	}
}

func TestReportEmptyWhenNoMatches(t *testing.T) {
	base := time.Now()
	entry := newCompletedEntry("old", "Archive", "Past task", base.Add(-30*24*time.Hour))
	mp := newMemoryPersistence(entry)
	svc := &Service{Persistence: mp}

	res, err := svc.Report(context.Background(), base.Add(-7*24*time.Hour), base)
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if res.Total != 0 {
		t.Fatalf("expected no matches, got %d", res.Total)
	}
	if len(res.Sections) != 0 {
		t.Fatalf("expected no sections, got %d", len(res.Sections))
	}
}

func newCompletedEntry(id, collection, message string, completedAt time.Time) *entry.Entry {
	e := &entry.Entry{
		ID:         id,
		Bullet:     glyph.Completed,
		Schema:     entry.CurrentSchema,
		Created:    entry.Timestamp{Time: completedAt.Add(-time.Hour)},
		Collection: collection,
		Message:    message,
		History: []entry.HistoryRecord{
			{
				Timestamp: entry.Timestamp{Time: completedAt.Add(-time.Hour)},
				Action:    entry.HistoryActionAdded,
				To:        collection,
			},
			{
				Timestamp: entry.Timestamp{Time: completedAt},
				Action:    entry.HistoryActionCompleted,
				To:        collection,
			},
		},
	}
	return e
}
