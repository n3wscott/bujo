package teaui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/v2/list"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/runner/tea/internal/indexview"
	"tableflip.dev/bujo/pkg/store"
)

func TestMoveCalendarCursorVertical(t *testing.T) {
	m := New(nil)
	m.focus = 0

	month := "November 2025"
	monthTime := time.Date(2025, time.November, 1, 0, 0, 0, 0, time.UTC)

	header, weeks := indexview.RenderCalendarRows(month, monthTime, nil, 1, monthTime, indexview.DefaultCalendarOptions())
	if header == nil || len(weeks) == 0 {
		t.Fatalf("expected calendar rows for month %s", month)
	}

	items := []list.Item{
		indexview.CollectionItem{Name: month, Resolved: month, HasChildren: true},
		header,
	}
	for _, w := range weeks {
		w.RowIndex = len(items)
		items = append(items, w)
	}

	state := &indexview.MonthState{
		Month:     month,
		MonthTime: monthTime,
		HeaderIdx: 1,
		Weeks:     weeks,
	}
	m.indexState.Months[month] = state
	m.indexState.Selection[month] = 1
	m.colList.SetItems(items)
	m.colList.Select(2) // first calendar week row

	cmd := m.moveCalendarCursor(0, 1)
	if cmd == nil {
		t.Fatalf("expected moveCalendarCursor to produce command")
	}

	if got := m.indexState.Selection[month]; got != 8 {
		t.Fatalf("expected selection to move to day 8, got %d", got)
	}
	if idx := m.colList.Index(); idx != 3 {
		t.Fatalf("expected list cursor to move to row index 3, got %d", idx)
	}
	if m.pendingResolved != indexview.FormatDayPath(monthTime, 8) {
		t.Fatalf("expected pendingResolved to point at day 8, got %q", m.pendingResolved)
	}
}

func TestToggleFoldCurrentFromParentAndChild(t *testing.T) {
	m := New(nil)
	m.focus = 0

	now := time.Date(2025, time.November, 5, 0, 0, 0, 0, time.UTC)
	cols := []string{
		"November 2025",
		"November 2025/November 2, 2025",
	}

	items := m.buildCollectionItems(cols, "", now)
	m.colList.SetItems(items)

	monthIdx := indexForName(m.colList.Items(), "November 2025")
	if monthIdx < 0 {
		t.Fatalf("month collection not found")
	}
	m.colList.Select(monthIdx)

	if collapsed := m.indexState.Fold["November 2025"]; collapsed {
		t.Fatalf("current month should be expanded by default")
	}

	if cmd := m.toggleFoldCurrent(nil); cmd == nil {
		t.Fatalf("expected toggleFoldCurrent to return command when collapsing")
	}
	if !m.indexState.Fold["November 2025"] {
		t.Fatalf("expected fold state to collapse after toggle")
	}

	m.colList.Select(monthIdx + 1) // select child day
	if cmd := m.toggleFoldCurrent(nil); cmd == nil {
		t.Fatalf("expected toggleFoldCurrent from child to return command")
	}
	if m.indexState.Fold["November 2025"] {
		t.Fatalf("expected fold state to expand when toggled from child")
	}
}

func TestLoadEntriesSortsByCreatedAscending(t *testing.T) {
	fp := &fakePersistence{
		data: map[string][]*entry.Entry{
			"Projects": {
				newEntryWithCreated("Projects", "Third", time.Date(2025, time.November, 3, 12, 0, 0, 0, time.UTC)),
				newEntryWithCreated("Projects", "First", time.Date(2025, time.November, 1, 12, 0, 0, 0, time.UTC)),
				newEntryWithCreated("Projects", "Second", time.Date(2025, time.November, 2, 12, 0, 0, 0, time.UTC)),
			},
		},
	}
	svc := &app.Service{Persistence: fp}

	m := New(svc)
	m.focus = 0
	m.colList.SetItems([]list.Item{indexview.CollectionItem{Name: "Projects", Resolved: "Projects"}})
	m.colList.Select(0)

	cmd := m.loadDetailSections()
	if cmd == nil {
		t.Fatalf("expected loadDetailSections to produce command")
	}
	msg := cmd()
	loaded, ok := msg.(detailSectionsLoadedMsg)
	if !ok {
		t.Fatalf("expected detailSectionsLoadedMsg, got %T", msg)
	}
	if len(loaded.sections) == 0 {
		t.Fatalf("expected at least one section")
	}
	entries := loaded.sections[0].Entries
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	var ordered []string
	for _, it := range entries {
		ordered = append(ordered, it.Message)
	}

	want := []string{"First", "Second", "Third"}
	for i, name := range want {
		if ordered[i] != name {
			t.Fatalf("order mismatch at %d: want %s, got %s", i, name, ordered[i])
		}
	}
}

type fakePersistence struct {
	data map[string][]*entry.Entry
}

func (f *fakePersistence) MapAll(ctx context.Context) map[string][]*entry.Entry {
	result := make(map[string][]*entry.Entry, len(f.data))
	for k, entries := range f.data {
		result[k] = append([]*entry.Entry(nil), entries...)
	}
	return result
}

func (f *fakePersistence) ListAll(ctx context.Context) []*entry.Entry {
	var all []*entry.Entry
	for _, entries := range f.data {
		all = append(all, entries...)
	}
	return append([]*entry.Entry(nil), all...)
}

func (f *fakePersistence) List(ctx context.Context, collection string) []*entry.Entry {
	return append([]*entry.Entry(nil), f.data[collection]...)
}

func (f *fakePersistence) Collections(ctx context.Context, prefix string) []string {
	var cols []string
	for col := range f.data {
		if prefix == "" || strings.HasPrefix(col, prefix) {
			cols = append(cols, col)
		}
	}
	return cols
}

func (f *fakePersistence) Store(e *entry.Entry) error {
	f.data[e.Collection] = append(f.data[e.Collection], e)
	return nil
}

func newEntryWithCreated(collection, message string, created time.Time) *entry.Entry {
	return &entry.Entry{
		Collection: collection,
		Message:    message,
		Bullet:     glyph.Task,
		Created:    entry.Timestamp{Time: created},
	}
}

var _ store.Persistence = (*fakePersistence)(nil)
