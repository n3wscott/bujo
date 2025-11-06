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

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/store"
)

type memoryPersistence struct {
	mu          sync.Mutex
	counter     int
	collections map[string]map[string]*entry.Entry
	types       map[string]collection.Type
}

func newMemoryPersistence(entries ...*entry.Entry) *memoryPersistence {
	mp := &memoryPersistence{
		collections: make(map[string]map[string]*entry.Entry),
		types:       make(map[string]collection.Type),
	}
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

func (m *memoryPersistence) CollectionsMeta(_ context.Context, prefix string) []collection.Meta {
	m.mu.Lock()
	defer m.mu.Unlock()
	metas := make([]collection.Meta, 0, len(m.collections))
	for name := range m.collections {
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		metas = append(metas, collection.Meta{
			Name: name,
			Type: m.types[name],
		})
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].Name < metas[j].Name })
	return metas
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

func (m *memoryPersistence) EnsureCollection(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("collection required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.collections[name] == nil {
		m.collections[name] = make(map[string]*entry.Entry)
	}
	if _, ok := m.types[name]; !ok {
		m.types[name] = collection.TypeGeneric
	}
	return nil
}

func (m *memoryPersistence) EnsureCollectionTyped(name string, typ collection.Type) error {
	if err := m.EnsureCollection(name); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if typ == "" {
		typ = collection.TypeGeneric
	}
	m.types[name] = typ
	return nil
}

func (m *memoryPersistence) DeleteCollection(_ context.Context, name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return errors.New("collection required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	prefix := trimmed + "/"
	removed := false
	for col := range m.collections {
		if col == trimmed || strings.HasPrefix(col, prefix) {
			delete(m.collections, col)
			delete(m.types, col)
			removed = true
		}
	}
	if !removed {
		return fmt.Errorf("collection %q not found", trimmed)
	}
	return nil
}

func TestMigrationCandidates_DefaultFilters(t *testing.T) {
	now := time.Date(2024, time.May, 15, 9, 0, 0, 0, time.UTC)
	task := &entry.Entry{
		ID:         "task-1",
		Collection: "Inbox",
		Bullet:     glyph.Task,
		Created:    entry.Timestamp{Time: now.Add(-24 * time.Hour)},
	}
	eventEntry := &entry.Entry{
		ID:         "event-1",
		Collection: "Meetings",
		Bullet:     glyph.Event,
		Created:    entry.Timestamp{Time: now.Add(-2 * time.Hour)},
	}
	completed := &entry.Entry{
		ID:         "done-1",
		Collection: "Inbox",
		Bullet:     glyph.Completed,
		Created:    entry.Timestamp{Time: now.Add(-time.Hour)},
	}
	note := &entry.Entry{
		ID:         "note-1",
		Collection: "Inbox",
		Bullet:     glyph.Note,
		Created:    entry.Timestamp{Time: now},
	}
	futureRoot := &entry.Entry{
		ID:         "future-root",
		Collection: "Future",
		Bullet:     glyph.Task,
		Created:    entry.Timestamp{Time: now.Add(-10 * 24 * time.Hour)},
	}
	futureMonth := &entry.Entry{
		ID:         "future-month",
		Collection: "Future/June 2024",
		Bullet:     glyph.Task,
		Created:    entry.Timestamp{Time: now.Add(-3 * 24 * time.Hour)},
	}
	futureDay := &entry.Entry{
		ID:         "future-day",
		Collection: "May 2024/May 20, 2024",
		Bullet:     glyph.Task,
		Created:    entry.Timestamp{Time: now.Add(-2 * 24 * time.Hour)},
	}
	pastDay := &entry.Entry{
		ID:         "past-day",
		Collection: "May 2024/May 10, 2024",
		Bullet:     glyph.Task,
		Created:    entry.Timestamp{Time: now.Add(-6 * 24 * time.Hour)},
	}
	locked := &entry.Entry{
		ID:         "locked",
		Collection: "Inbox",
		Bullet:     glyph.Task,
		Created:    entry.Timestamp{Time: now.Add(-3 * time.Hour)},
		Immutable:  true,
	}

	store := newMemoryPersistence(task, eventEntry, completed, note, futureRoot, futureMonth, futureDay, pastDay, locked)
	svc := &Service{Persistence: store}

	results, err := svc.MigrationCandidates(context.Background(), time.Time{}, now)
	if err != nil {
		t.Fatalf("MigrationCandidates error: %v", err)
	}
	got := make(map[string]bool)
	for _, cand := range results {
		if cand.Entry != nil {
			got[cand.Entry.ID] = true
		}
	}
	assertIncluded := func(id string) {
		if !got[id] {
			t.Fatalf("expected %q to be included, but was missing (results=%v)", id, got)
		}
	}
	assertExcluded := func(id string) {
		if got[id] {
			t.Fatalf("expected %q to be excluded, but found (results=%v)", id, got)
		}
	}

	assertIncluded("task-1")
	assertIncluded("event-1")
	assertIncluded("future-root")
	assertIncluded("past-day")

	assertExcluded("done-1")
	assertExcluded("note-1")
	assertExcluded("future-month")
	assertExcluded("future-day")
	assertExcluded("locked")
}

func TestMigrationCandidates_WindowFilters(t *testing.T) {
	now := time.Date(2024, time.April, 30, 12, 0, 0, 0, time.UTC)
	recent := &entry.Entry{
		ID:         "recent",
		Collection: "Inbox",
		Bullet:     glyph.Task,
		Created:    entry.Timestamp{Time: now.Add(-2 * time.Hour)},
	}
	old := &entry.Entry{
		ID:         "old",
		Collection: "Inbox",
		Bullet:     glyph.Task,
		Created:    entry.Timestamp{Time: now.Add(-10 * 24 * time.Hour)},
	}
	futureRoot := &entry.Entry{
		ID:         "future-root",
		Collection: "Future",
		Bullet:     glyph.Task,
		Created:    entry.Timestamp{Time: now.Add(-30 * 24 * time.Hour)},
	}
	store := newMemoryPersistence(recent, old, futureRoot)
	svc := &Service{Persistence: store}

	since := now.Add(-72 * time.Hour)
	results, err := svc.MigrationCandidates(context.Background(), since, now)
	if err != nil {
		t.Fatalf("MigrationCandidates error: %v", err)
	}
	got := make(map[string]bool)
	for _, cand := range results {
		if cand.Entry != nil {
			got[cand.Entry.ID] = true
		}
	}
	if !got["recent"] {
		t.Fatalf("expected \"recent\" to be included within window, got %v", got)
	}
	if got["old"] {
		t.Fatalf("expected \"old\" to be excluded outside window, got %v", got)
	}
	if !got["future-root"] {
		t.Fatalf("expected \"future-root\" to be included regardless of window, got %v", got)
	}
}

func (m *memoryPersistence) SetCollectionType(name string, typ collection.Type) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.collections[name]; !ok {
		return errors.New("unknown collection")
	}
	m.types[name] = typ
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

func TestEnsureCollectionsInfersCalendarTypes(t *testing.T) {
	mp := newMemoryPersistence()
	svc := Service{Persistence: mp}
	ctx := context.Background()

	if err := svc.EnsureCollections(ctx, []string{"Future"}); err != nil {
		t.Fatalf("EnsureCollections(Future): %v", err)
	}
	if got := mp.types["Future"]; got != collection.TypeMonthly {
		t.Fatalf("expected Future to be monthly, got %s", got)
	}

	if err := svc.EnsureCollections(ctx, []string{"Future/October 2025"}); err != nil {
		t.Fatalf("EnsureCollections(Future/October 2025): %v", err)
	}
	if got := mp.types["Future/October 2025"]; got != collection.TypeDaily {
		t.Fatalf("expected Future/October 2025 to be daily, got %s", got)
	}
}

func TestSetCollectionTypeValidatesChildren(t *testing.T) {
	mp := newMemoryPersistence()
	if err := mp.EnsureCollection("Future"); err != nil {
		t.Fatalf("ensure parent: %v", err)
	}
	if err := mp.EnsureCollection("Future/Projects"); err != nil {
		t.Fatalf("ensure child: %v", err)
	}
	svc := Service{Persistence: mp}
	err := svc.SetCollectionType(context.Background(), "Future", collection.TypeMonthly)
	if err == nil {
		t.Fatalf("expected error when assigning monthly type to invalid children")
	}
	if !strings.Contains(err.Error(), "only accepts month children") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureCollectionOfTypeCreatesAncestors(t *testing.T) {
	mp := newMemoryPersistence()
	svc := Service{Persistence: mp}
	ctx := context.Background()

	if err := svc.EnsureCollectionOfType(ctx, "Future/January 2026", collection.TypeDaily); err != nil {
		t.Fatalf("EnsureCollectionOfType: %v", err)
	}
	if got := mp.types["Future"]; got != collection.TypeMonthly {
		t.Fatalf("expected Future to be monthly, got %s", got)
	}
	if got := mp.types["Future/January 2026"]; got != collection.TypeDaily {
		t.Fatalf("expected child to be daily, got %s", got)
	}
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

func TestReportIncludesParentEntries(t *testing.T) {
	base := time.Now()
	parent := &entry.Entry{
		ID:         "p",
		Bullet:     glyph.Task,
		Schema:     entry.CurrentSchema,
		Created:    entry.Timestamp{Time: base.Add(-48 * time.Hour)},
		Collection: "Today",
		Message:    "Parent task",
	}
	child := newCompletedEntry("c", "Today", "Child task", base.Add(-2*time.Hour))
	child.ParentID = parent.ID
	mp := newMemoryPersistence(parent, child)
	svc := &Service{Persistence: mp}

	res, err := svc.Report(context.Background(), base.Add(-24*time.Hour), base)
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if res.Total != 1 {
		t.Fatalf("expected 1 completed entry, got %d", res.Total)
	}
	if len(res.Sections) != 1 {
		t.Fatalf("expected a single section, got %d", len(res.Sections))
	}
	entries := res.Sections[0].Entries
	if len(entries) != 2 {
		t.Fatalf("expected parent and child entries, got %d", len(entries))
	}
	var seenParent, seenChild bool
	for _, item := range entries {
		if item.Entry == nil {
			continue
		}
		switch item.Entry.ID {
		case parent.ID:
			seenParent = true
			if item.Completed {
				t.Fatalf("parent should not be marked completed")
			}
		case child.ID:
			seenChild = true
			if !item.Completed {
				t.Fatalf("child should be marked completed")
			}
		default:
			t.Fatalf("unexpected entry %s present in report", item.Entry.ID)
		}
	}
	if !seenParent || !seenChild {
		t.Fatalf("expected both parent (%v) and child (%v) entries", seenParent, seenChild)
	}
}

func TestMigrationCandidatesFiltersOpenTasks(t *testing.T) {
	ctx := context.Background()
	base := time.Date(2025, time.November, 10, 12, 0, 0, 0, time.UTC)

	parent := &entry.Entry{
		ID:         "parent",
		Collection: "Inbox",
		Message:    "Project Kickoff",
		Bullet:     glyph.Task,
		Schema:     entry.CurrentSchema,
		Created:    entry.Timestamp{Time: base.Add(-30 * 24 * time.Hour)},
	}
	parent.EnsureHistorySeed()

	recent := &entry.Entry{
		ID:         "recent",
		Collection: "Inbox",
		ParentID:   parent.ID,
		Message:    "Finalize agenda",
		Bullet:     glyph.Task,
		Schema:     entry.CurrentSchema,
		Created:    entry.Timestamp{Time: base.Add(-6 * 24 * time.Hour)},
	}
	recent.EnsureHistorySeed()
	recent.History = append(recent.History, entry.HistoryRecord{
		Timestamp: entry.Timestamp{Time: base.Add(-2 * time.Hour)},
		Action:    entry.HistoryActionMoved,
		From:      "Inbox",
		To:        "Inbox",
	})

	lessRecent := &entry.Entry{
		ID:         "less",
		Collection: "Personal",
		Message:    "Call dentist",
		Bullet:     glyph.Task,
		Schema:     entry.CurrentSchema,
		Created:    entry.Timestamp{Time: base.Add(-5 * 24 * time.Hour)},
	}
	lessRecent.EnsureHistorySeed()
	lessRecent.History = append(lessRecent.History, entry.HistoryRecord{
		Timestamp: entry.Timestamp{Time: base.Add(-3 * time.Hour)},
		Action:    entry.HistoryActionAdded,
		To:        "Personal",
	})

	outOfRange := &entry.Entry{
		ID:         "old",
		Collection: "Inbox",
		Message:    "Old task",
		Bullet:     glyph.Task,
		Schema:     entry.CurrentSchema,
		Created:    entry.Timestamp{Time: base.Add(-60 * 24 * time.Hour)},
	}
	outOfRange.EnsureHistorySeed()
	outOfRange.History = append(outOfRange.History, entry.HistoryRecord{
		Timestamp: entry.Timestamp{Time: base.Add(-20 * 24 * time.Hour)},
		Action:    entry.HistoryActionMoved,
		To:        "Inbox",
	})

	completed := &entry.Entry{
		ID:         "done",
		Collection: "Inbox",
		Message:    "Already done",
		Bullet:     glyph.Completed,
		Schema:     entry.CurrentSchema,
		Created:    entry.Timestamp{Time: base.Add(-3 * 24 * time.Hour)},
	}
	completed.EnsureHistorySeed()
	completed.History = append(completed.History, entry.HistoryRecord{
		Timestamp: entry.Timestamp{Time: base.Add(-10 * time.Hour)},
		Action:    entry.HistoryActionCompleted,
	})

	mp := newMemoryPersistence(parent, recent, lessRecent, outOfRange, completed)
	svc := &Service{Persistence: mp}

	since := base.Add(-7 * 24 * time.Hour)
	candidates, err := svc.MigrationCandidates(ctx, since, base)
	if err != nil {
		t.Fatalf("MigrationCandidates: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if candidates[0].Entry.ID != recent.ID {
		t.Fatalf("expected first candidate to be %s, got %s", recent.ID, candidates[0].Entry.ID)
	}
	if candidates[1].Entry.ID != lessRecent.ID {
		t.Fatalf("expected second candidate to be %s, got %s", lessRecent.ID, candidates[1].Entry.ID)
	}
	if candidates[0].Parent == nil || candidates[0].Parent.ID != parent.ID {
		t.Fatalf("expected parent context for recent task")
	}
	if candidates[0].LastTouched.After(base) || candidates[0].LastTouched.Before(since) {
		t.Fatalf("expected last touched within window, got %v", candidates[0].LastTouched)
	}
}

func TestDeleteCollectionRemovesDescendants(t *testing.T) {
	ctx := context.Background()
	parent := entry.New("Future", glyph.Task, "parent task")
	child := entry.New("Future/November 2025", glyph.Task, "child task")
	mp := newMemoryPersistence(parent, child)
	svc := &Service{Persistence: mp}
	if err := svc.DeleteCollection(ctx, "Future"); err != nil {
		t.Fatalf("DeleteCollection: %v", err)
	}
	if entries := mp.List(ctx, "Future"); len(entries) != 0 {
		t.Fatalf("expected parent collection removed, still have %d entries", len(entries))
	}
	if entries := mp.List(ctx, "Future/November 2025"); len(entries) != 0 {
		t.Fatalf("expected child collection removed, still have %d entries", len(entries))
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
