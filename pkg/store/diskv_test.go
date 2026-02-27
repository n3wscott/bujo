package store

import (
	"context"
	"strconv"
	"testing"
	"time"

	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
)

type diskvTestConfig struct{ base string }

func (c diskvTestConfig) BasePath() string { return c.base }

func newTestPersistence(t *testing.T) *persistence {
	t.Helper()
	p, err := Load(diskvTestConfig{base: t.TempDir()})
	if err != nil {
		t.Fatalf("load persistence: %v", err)
	}
	return p.(*persistence)
}

func TestStoreAndListRoundTrip(t *testing.T) {
	p := newTestPersistence(t)
	ctx := context.Background()
	entry := entry.New("Inbox", glyph.Task, "hello")
	if err := p.Store(entry); err != nil {
		t.Fatalf("store entry: %v", err)
	}
	list := p.List(ctx, "Inbox")
	if len(list) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(list))
	}
	if list[0].Message != "hello" {
		t.Fatalf("expected message to round trip, got %q", list[0].Message)
	}
	if list[0].ID == "" {
		t.Fatalf("expected stored entry to have ID")
	}
}

func TestListSkipsCorruptEntries(t *testing.T) {
	p := newTestPersistence(t)
	ctx := context.Background()
	good := entry.New("Inbox", glyph.Task, "ok")
	good.Created = entry.Timestamp{Time: time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)}
	if err := p.Store(good); err != nil {
		t.Fatalf("store entry: %v", err)
	}

	corrupt := &entry.Entry{Collection: "Inbox", Created: entry.Timestamp{Time: time.Date(2024, time.January, 2, 0, 0, 0, 0, time.UTC)}, ID: "corrupt"}
	if err := p.d.Write(toKey(corrupt), []byte("{not-json")); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}

	list := p.ListAll(ctx)
	if len(list) != 1 {
		t.Fatalf("expected 1 valid entry, got %d", len(list))
	}
	if list[0].Message != "ok" {
		t.Fatalf("expected valid entry to remain, got %q", list[0].Message)
	}
}

func TestDeleteCollectionRemovesEntries(t *testing.T) {
	p := newTestPersistence(t)
	ctx := context.Background()
	entryA := entry.New("Inbox", glyph.Task, "hello")
	entryB := entry.New("Other", glyph.Task, "world")
	if err := p.Store(entryA); err != nil {
		t.Fatalf("store entry: %v", err)
	}
	if err := p.Store(entryB); err != nil {
		t.Fatalf("store entry: %v", err)
	}
	if err := p.DeleteCollection(ctx, "Inbox"); err != nil {
		t.Fatalf("delete collection: %v", err)
	}
	if list := p.List(ctx, "Inbox"); len(list) != 0 {
		t.Fatalf("expected inbox to be empty after delete")
	}
	if list := p.List(ctx, "Other"); len(list) != 1 {
		t.Fatalf("expected other collection to remain")
	}
}

func TestDeleteEntryAcrossTimezoneBoundary(t *testing.T) {
	p := newTestPersistence(t)
	ctx := context.Background()
	pst := time.FixedZone("PST", -8*60*60)
	e := entry.New("Inbox", glyph.Task, "late task")
	e.Created = entry.Timestamp{Time: time.Date(2026, time.February, 26, 23, 30, 0, 0, pst)}
	if err := p.Store(e); err != nil {
		t.Fatalf("store entry: %v", err)
	}

	all := p.ListAll(ctx)
	if len(all) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(all))
	}
	if err := p.Delete(all[0]); err != nil {
		t.Fatalf("delete entry: %v", err)
	}
	if remaining := p.ListAll(ctx); len(remaining) != 0 {
		t.Fatalf("expected no entries after delete, got %d", len(remaining))
	}
}

func TestConcurrentStore(t *testing.T) {
	p := newTestPersistence(t)
	ctx := context.Background()
	const total = 10
	done := make(chan struct{}, total)
	for i := 0; i < total; i++ {
		go func(idx int) {
			defer func() { done <- struct{}{} }()
			e := entry.New("Inbox", glyph.Task, "task")
			e.Message = e.Message + "-" + strconv.Itoa(idx)
			_ = p.Store(e)
		}(i)
	}
	for i := 0; i < total; i++ {
		<-done
	}
	list := p.List(ctx, "Inbox")
	if len(list) != total {
		t.Fatalf("expected %d entries after concurrent store, got %d", total, len(list))
	}
}
