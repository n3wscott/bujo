package teaui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/muesli/reflow/ansi"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/runner/tea/internal/detailview"
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

func TestDetailSectionsHideMovedImmutableByDefault(t *testing.T) {
	now := time.Date(2025, time.November, 5, 10, 0, 0, 0, time.UTC)
	locked := newEntryWithCreated("Projects", "Legacy", now.Add(-2*time.Hour))
	locked.ID = "locked"
	locked.Bullet = glyph.MovedCollection
	locked.Immutable = true

	fp := &fakePersistence{
		data: map[string][]*entry.Entry{
			"Projects": {locked},
		},
	}
	svc := &app.Service{Persistence: fp}

	m := New(svc)
	m.focus = 1
	m.colList.SetItems([]list.Item{indexview.CollectionItem{Name: "Projects", Resolved: "Projects"}})
	m.colList.Select(0)
	drain := func(queue []tea.Cmd) {
		for len(queue) > 0 {
			next := queue[0]
			queue = queue[1:]
			if next == nil {
				continue
			}
			msg := next()
			if msg == nil {
				continue
			}
			var add tea.Cmd
			var model tea.Model
			model, add = m.Update(msg)
			m = model.(*Model)
			if add != nil {
				queue = append(queue, add)
			}
		}
	}

	cmd := m.loadDetailSectionsWithFocus("Projects", "")
	if cmd == nil {
		t.Fatalf("expected loadDetailSectionsWithFocus command")
	}
	msg := cmd()
	loaded, ok := msg.(detailSectionsLoadedMsg)
	if !ok {
		t.Fatalf("expected detailSectionsLoadedMsg, got %T", msg)
	}
	model, follow := m.Update(msg)
	m = model.(*Model)
	if follow != nil {
		drain([]tea.Cmd{follow})
	}
	if len(loaded.sections) != 1 {
		t.Fatalf("expected one empty section when entries hidden, got %d", len(loaded.sections))
	}
	if entries := loaded.sections[0].Entries; len(entries) != 0 {
		t.Fatalf("expected hidden section to contain no entries, got %d", len(entries))
	}
	if idx := indexForResolved(m.colList.Items(), "Projects"); idx == -1 {
		t.Fatalf("expected Projects to remain in index for active focus")
	}

	m.showHiddenMoved = true
	drain([]tea.Cmd{m.loadCollections(), m.loadDetailSectionsWithFocus("Projects", "")})

	sections := m.detailState.Sections()
	if len(sections) != 1 {
		t.Fatalf("expected one section when showing hidden, got %d", len(sections))
	}
	entries := sections[0].Entries
	if len(entries) != 1 {
		t.Fatalf("expected hidden entry to appear when showing hidden, got %d entries", len(entries))
	}
	if entries[0].ID != "locked" {
		t.Fatalf("unexpected entry visible: %q", entries[0].ID)
	}
	if idx := indexForResolved(m.colList.Items(), "Projects"); idx == -1 {
		t.Fatalf("expected Projects to remain in index when showing hidden")
	}
}

func TestExecuteCommandShowHiddenOn(t *testing.T) {
	svc := &app.Service{Persistence: &fakePersistence{data: map[string][]*entry.Entry{}}}
	m := New(svc)
	m.mode = modeCommand

	var cmds []tea.Cmd
	m.executeCommand("show-hidden on", &cmds)

	if !m.showHiddenMoved {
		t.Fatalf("expected showHiddenMoved to be true after command")
	}
	if m.mode != modeNormal {
		t.Fatalf("expected mode to reset to normal, got %v", m.mode)
	}
	if len(cmds) < 2 {
		t.Fatalf("expected loadCollections and loadDetailSections commands, got %d", len(cmds))
	}
}

func TestMkdirCommandCreatesHierarchy(t *testing.T) {
	fp := &fakePersistence{data: map[string][]*entry.Entry{}}
	svc := &app.Service{Persistence: fp}
	m := New(svc)
	m.mode = modeCommand

	var cmds []tea.Cmd
	m.executeCommand("mkdir parent/child", &cmds)

	if _, ok := fp.collections["parent"]; !ok {
		t.Fatalf("expected parent collection to be created")
	}
	if _, ok := fp.collections["parent/child"]; !ok {
		t.Fatalf("expected child collection to be created")
	}
	if m.mode != modeNormal {
		t.Fatalf("expected mode to reset to normal, got %v", m.mode)
	}
}

func TestApplyEditImmutableSetsStatus(t *testing.T) {
	now := time.Now()
	locked := newEntryWithCreated("Inbox", "Locked item", now)
	locked.ID = "item"
	locked.Immutable = true

	fp := &fakePersistence{
		data: map[string][]*entry.Entry{
			"Inbox": {locked},
		},
	}
	svc := &app.Service{Persistence: fp}
	m := New(svc)

	var cmds []tea.Cmd
	m.applyEdit(&cmds, "item", "updated")
	if len(cmds) != 0 {
		t.Fatalf("expected no commands when edit blocked, got %d", len(cmds))
	}
	view, _ := m.bottom.View()
	if !strings.Contains(stripANSI(view), "Entry is locked") {
		t.Fatalf("expected locked status message, got %q", view)
	}
}

func TestDetailActiveAlignsCollectionSelection(t *testing.T) {
	fp := &fakePersistence{
		data: map[string][]*entry.Entry{
			"Today":    {newEntryWithCreated("Today", "root", time.Now())},
			"Tomorrow": {newEntryWithCreated("Tomorrow", "later", time.Now())},
		},
	}
	svc := &app.Service{Persistence: fp}

	m := New(svc)
	m.focus = 1
	m.colList.SetItems([]list.Item{
		indexview.CollectionItem{Name: "Today", Resolved: "Today"},
		indexview.CollectionItem{Name: "Tomorrow", Resolved: "Tomorrow"},
	})
	m.colList.Select(0)
	m.colList.Select(1)
	if idx := m.colList.Index(); idx != 1 {
		t.Fatalf("expected manual select to update index to 1, got %d", idx)
	}
	m.colList.Select(0)
	cmds := []tea.Cmd{}
	m.alignCollectionSelection("Tomorrow", &cmds)
	if idx := m.colList.Index(); idx != 1 {
		t.Fatalf("align helper failed to update index, got %d", idx)
	}
	// reset for test scenario
	m.colList.Select(0)

	sections := []detailview.Section{
		{CollectionID: "Today", CollectionName: "Today", Entries: fp.data["Today"]},
		{CollectionID: "Tomorrow", CollectionName: "Tomorrow", Entries: fp.data["Tomorrow"]},
	}

	msg := detailSectionsLoadedMsg{sections: sections, activeCollection: "Tomorrow", activeEntry: ""}
	model, _ := m.Update(msg)
	m = model.(*Model)
	if active := m.detailState.ActiveCollectionID(); active != "Tomorrow" {
		t.Fatalf("expected detail active collection 'Tomorrow', got %q", active)
	}
	if idx := indexForResolved(m.colList.Items(), "Tomorrow"); idx != 1 {
		t.Fatalf("expected resolved lookup to find index 1, got %d", idx)
	}

	if idx := m.colList.Index(); idx != 1 {
		t.Fatalf("expected collection selection to follow detail focus; got index %d", idx)
	}
}

func TestAlignCollectionSelectionCalendarDay(t *testing.T) {
	m := New(nil)
	month := "October 2025"
	monthTime := time.Date(2025, time.October, 1, 0, 0, 0, 0, time.UTC)

	header, weeks := indexview.RenderCalendarRows(month, monthTime, nil, 1, monthTime, indexview.DefaultCalendarOptions())
	if header == nil || len(weeks) == 0 {
		t.Fatalf("expected calendar rows")
	}
	items := []list.Item{
		indexview.CollectionItem{Name: month, Resolved: month, HasChildren: true},
		header,
	}
	for _, w := range weeks {
		w.RowIndex = len(items)
		items = append(items, w)
	}
	state := &indexview.MonthState{Month: month, MonthTime: monthTime, HeaderIdx: 1, Weeks: weeks}
	m.indexState.Months[month] = state
	m.colList.SetItems(items)
	m.colList.Select(2)

	day := 5
	resolved := indexview.FormatDayPath(monthTime, day)
	var cmds []tea.Cmd
	m.alignCollectionSelection(resolved, &cmds)

	if m.indexState.Selection[month] != day {
		t.Fatalf("expected index state selection %d, got %d", day, m.indexState.Selection[month])
	}
	selected := m.colList.SelectedItem()
	week, ok := selected.(*indexview.CalendarRowItem)
	if !ok {
		t.Fatalf("expected calendar row selection, got %T", selected)
	}
	if !indexview.ContainsDay(week.Days, day) {
		t.Fatalf("selected week does not contain day %d", day)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	ansiSeq := false
	for _, r := range s {
		if r == ansi.Marker {
			ansiSeq = true
			continue
		}
		if ansiSeq {
			if ansi.IsTerminator(r) {
				ansiSeq = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

type fakePersistence struct {
	data        map[string][]*entry.Entry
	collections map[string]struct{}
	types       map[string]collection.Type
}

func (f *fakePersistence) MapAll(_ context.Context) map[string][]*entry.Entry {
	result := make(map[string][]*entry.Entry, len(f.data))
	for k, entries := range f.data {
		result[k] = append([]*entry.Entry(nil), entries...)
	}
	return result
}

func (f *fakePersistence) ListAll(_ context.Context) []*entry.Entry {
	var all []*entry.Entry
	for _, entries := range f.data {
		all = append(all, entries...)
	}
	return append([]*entry.Entry(nil), all...)
}

func (f *fakePersistence) List(_ context.Context, collection string) []*entry.Entry {
	return append([]*entry.Entry(nil), f.data[collection]...)
}

func (f *fakePersistence) Collections(_ context.Context, prefix string) []string {
	seen := make(map[string]struct{})
	var cols []string
	for col := range f.data {
		if prefix == "" || strings.HasPrefix(col, prefix) {
			if _, ok := seen[col]; !ok {
				seen[col] = struct{}{}
				cols = append(cols, col)
			}
		}
	}
	for col := range f.collections {
		if prefix == "" || strings.HasPrefix(col, prefix) {
			if _, ok := seen[col]; !ok {
				seen[col] = struct{}{}
				cols = append(cols, col)
			}
		}
	}
	return cols
}

func (f *fakePersistence) CollectionsMeta(_ context.Context, prefix string) []collection.Meta {
	var metas []collection.Meta
	seen := make(map[string]struct{})
	for name := range f.collections {
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		metas = append(metas, collection.Meta{Name: name, Type: f.types[name]})
	}
	for name := range f.data {
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		metas = append(metas, collection.Meta{Name: name, Type: f.types[name]})
	}
	return metas
}

func (f *fakePersistence) Store(e *entry.Entry) error {
	e.EnsureHistorySeed()
	entries := f.data[e.Collection]
	if e.ID != "" {
		for i, existing := range entries {
			if existing != nil && existing.ID == e.ID {
				entries[i] = e
				f.data[e.Collection] = entries
				return nil
			}
		}
	}
	f.data[e.Collection] = append(entries, e)
	if f.collections == nil {
		f.collections = make(map[string]struct{})
	}
	f.collections[e.Collection] = struct{}{}
	return nil
}

func (f *fakePersistence) Delete(e *entry.Entry) error {
	if e == nil {
		return nil
	}
	entries := f.data[e.Collection]
	for i, existing := range entries {
		if existing.ID == e.ID {
			f.data[e.Collection] = append(entries[:i], entries[i+1:]...)
			break
		}
	}
	return nil
}

func (f *fakePersistence) Watch(ctx context.Context) (<-chan store.Event, error) {
	ch := make(chan store.Event)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

func (f *fakePersistence) EnsureCollection(name string) error {
	if f.collections == nil {
		f.collections = make(map[string]struct{})
	}
	f.collections[name] = struct{}{}
	if f.types == nil {
		f.types = make(map[string]collection.Type)
	}
	if _, ok := f.types[name]; !ok {
		f.types[name] = collection.TypeGeneric
	}
	return nil
}

func (f *fakePersistence) EnsureCollectionTyped(name string, typ collection.Type) error {
	if err := f.EnsureCollection(name); err != nil {
		return err
	}
	if f.types == nil {
		f.types = make(map[string]collection.Type)
	}
	if typ == "" {
		typ = collection.TypeGeneric
	}
	f.types[name] = typ
	return nil
}

func (f *fakePersistence) SetCollectionType(name string, typ collection.Type) error {
	if f.types == nil {
		f.types = make(map[string]collection.Type)
	}
	if _, ok := f.collections[name]; !ok {
		return errors.New("unknown collection")
	}
	f.types[name] = typ
	return nil
}

func newEntryWithCreated(collection, message string, created time.Time) *entry.Entry {
	e := &entry.Entry{
		Collection: collection,
		Message:    message,
		Bullet:     glyph.Task,
		Created:    entry.Timestamp{Time: created},
	}
	e.EnsureHistorySeed()
	return e
}

var _ store.Persistence = (*fakePersistence)(nil)
