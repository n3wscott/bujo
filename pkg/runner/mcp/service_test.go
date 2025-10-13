package mcp

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
)

type memoryStore struct {
	entries map[string]*entry.Entry
	counter int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		entries: make(map[string]*entry.Entry),
	}
}

func (m *memoryStore) MapAll(ctx context.Context) map[string][]*entry.Entry {
	out := make(map[string][]*entry.Entry)
	for _, e := range m.entries {
		out[e.Collection] = append(out[e.Collection], e)
	}
	return out
}

func (m *memoryStore) ListAll(ctx context.Context) []*entry.Entry {
	out := make([]*entry.Entry, 0, len(m.entries))
	for _, e := range m.entries {
		out = append(out, e)
	}
	return out
}

func (m *memoryStore) List(ctx context.Context, collection string) []*entry.Entry {
	out := make([]*entry.Entry, 0)
	for _, e := range m.entries {
		if e.Collection == collection {
			out = append(out, e)
		}
	}
	return out
}

func (m *memoryStore) Collections(ctx context.Context, prefix string) []string {
	set := map[string]struct{}{}
	for _, e := range m.entries {
		if prefix == "" || strings.HasPrefix(e.Collection, prefix) {
			set[e.Collection] = struct{}{}
		}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	return names
}

func (m *memoryStore) Store(e *entry.Entry) error {
	if e.ID == "" {
		m.counter++
		e.ID = formatID(m.counter)
	}
	m.entries[e.ID] = e
	return nil
}

func formatID(i int) string {
	return "mcp-" + strconv.Itoa(i)
}

func TestServiceAddEntryDefaults(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	svc := NewService(store)

	dto, err := svc.AddEntry(ctx, AddEntryOptions{
		Collection: "Inbox",
		Message:    "Test item",
	})
	if err != nil {
		t.Fatalf("AddEntry failed: %v", err)
	}
	if dto.Collection != "Inbox" {
		t.Fatalf("expected collection Inbox, got %s", dto.Collection)
	}
	if dto.Bullet != string(glyph.Task) {
		t.Fatalf("expected task bullet, got %s", dto.Bullet)
	}
	if dto.Signifier != string(glyph.None) {
		t.Fatalf("expected none signifier, got %s", dto.Signifier)
	}
	if dto.ID == "" {
		t.Fatalf("expected generated id")
	}
}

func TestServiceCompleteEntry(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	svc := NewService(store)

	dto, err := svc.AddEntry(ctx, AddEntryOptions{
		Collection: "Today",
		Message:    "Finish report",
	})
	if err != nil {
		t.Fatalf("AddEntry failed: %v", err)
	}

	completed, err := svc.CompleteEntry(ctx, dto.ID)
	if err != nil {
		t.Fatalf("CompleteEntry failed: %v", err)
	}

	if !completed.IsCompleted {
		t.Fatalf("expected entry to be completed")
	}
	if completed.Bullet != string(glyph.Completed) {
		t.Fatalf("expected completed bullet, got %s", completed.Bullet)
	}
}
