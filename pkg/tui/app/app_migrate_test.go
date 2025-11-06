package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/store"
	"tableflip.dev/bujo/pkg/tui/components/command"
)

func TestShowMigrateOverlayIncludesRecentEntries(t *testing.T) {
	t.Helper()

	now := time.Now()
	recent := now.Add(-time.Minute)

	persistence := &migrateTestPersistence{
		entries: []*entry.Entry{
			{
				ID:         "recent",
				Collection: "Inbox",
				Bullet:     glyph.Task,
				Message:    "Migrate me",
				Created:    entry.Timestamp{Time: recent},
			},
		},
		metas: []collection.Meta{
			{Name: "Inbox", Type: collection.TypeGeneric},
			{Name: "Future", Type: collection.TypeMonthly},
		},
	}

	model := &Model{
		service: &app.Service{Persistence: persistence},
		command: command.NewModel(command.Options{}),
		width:   80,
		height:  24,
		today:   startOfDay(now),
	}

	if _, state := model.showMigrateOverlay(""); state != "opened" {
		t.Fatalf("expected overlay state \"opened\", got %q", state)
	}
	if model.migrateOverlay == nil {
		t.Fatalf("expected migrate overlay to be initialized")
	}
	if model.migrateOverlay.IsEmpty() {
		t.Fatalf("expected migrate overlay to include recent entries")
	}
}

type migrateTestPersistence struct {
	entries []*entry.Entry
	metas   []collection.Meta
}

func (p *migrateTestPersistence) MapAll(ctx context.Context) map[string][]*entry.Entry {
	out := make(map[string][]*entry.Entry)
	for _, e := range p.entries {
		if e == nil {
			continue
		}
		col := strings.TrimSpace(e.Collection)
		out[col] = append(out[col], cloneEntry(e))
	}
	return out
}

func (p *migrateTestPersistence) ListAll(ctx context.Context) []*entry.Entry {
	out := make([]*entry.Entry, 0, len(p.entries))
	for _, e := range p.entries {
		out = append(out, cloneEntry(e))
	}
	return out
}

func (p *migrateTestPersistence) List(ctx context.Context, collection string) []*entry.Entry {
	trimmed := strings.TrimSpace(collection)
	out := make([]*entry.Entry, 0)
	for _, e := range p.entries {
		if e == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(e.Collection), trimmed) {
			out = append(out, cloneEntry(e))
		}
	}
	return out
}

func (p *migrateTestPersistence) Collections(ctx context.Context, prefix string) []string {
	meta := p.CollectionsMeta(ctx, prefix)
	names := make([]string, 0, len(meta))
	for _, m := range meta {
		names = append(names, m.Name)
	}
	return names
}

func (p *migrateTestPersistence) CollectionsMeta(ctx context.Context, prefix string) []collection.Meta {
	out := make([]collection.Meta, 0, len(p.metas))
	for _, m := range p.metas {
		if prefix == "" || strings.HasPrefix(strings.ToLower(m.Name), strings.ToLower(prefix)) {
			out = append(out, m)
		}
	}
	return out
}

func (p *migrateTestPersistence) Store(e *entry.Entry) error {
	return nil
}

func (p *migrateTestPersistence) Delete(e *entry.Entry) error {
	return nil
}

func (p *migrateTestPersistence) DeleteCollection(ctx context.Context, collection string) error {
	return nil
}

func (p *migrateTestPersistence) EnsureCollection(collection string) error {
	return nil
}

func (p *migrateTestPersistence) EnsureCollectionTyped(collection string, typ collection.Type) error {
	return nil
}

func (p *migrateTestPersistence) SetCollectionType(collection string, typ collection.Type) error {
	return nil
}

func (p *migrateTestPersistence) Watch(ctx context.Context) (<-chan store.Event, error) {
	return nil, errors.New("not implemented")
}

func cloneEntry(e *entry.Entry) *entry.Entry {
	if e == nil {
		return nil
	}
	cp := *e
	if len(e.History) > 0 {
		cp.History = append([]entry.HistoryRecord(nil), e.History...)
	}
	return &cp
}
