package collectionnav

import (
	"strings"
	"testing"
	"time"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/collection/viewmodel"
	indexview "tableflip.dev/bujo/pkg/tui/components/index"
	"tableflip.dev/bujo/pkg/tui/events"
)

func TestViewTrimsCalendarPadding(t *testing.T) {
	day := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)

	month := &viewmodel.ParsedCollection{
		ID:       "Journal/January 2024",
		Name:     "January 2024",
		Type:     collection.TypeDaily,
		Exists:   true,
		ParentID: "Journal",
		Depth:    1,
		Month:    day,
		Days: []viewmodel.DaySummary{
			{
				ID:   "Journal/January 2024/January 1, 2024",
				Name: "January 1, 2024",
				Date: day,
			},
		},
	}
	month.Children = []*viewmodel.ParsedCollection{
		{
			ID:       "Journal/January 2024/January 1, 2024",
			Name:     "January 1, 2024",
			Type:     collection.TypeGeneric,
			Exists:   true,
			ParentID: month.ID,
			Depth:    2,
			Day:      day,
		},
	}

	root := &viewmodel.ParsedCollection{
		ID:       "Journal",
		Name:     "Journal",
		Type:     collection.TypeGeneric,
		Exists:   true,
		Children: []*viewmodel.ParsedCollection{month},
	}

	model := NewModel([]*viewmodel.ParsedCollection{root})
	model.SetSize(32, 8)
	model.SetNow(day)
	_ = model.SelectCollection(events.CollectionRef{ID: month.ID, Name: month.Name, Type: month.Type})

	view := model.View()
	lines := strings.Count(view, "\n") + 1
	if lines != 8 {
		t.Fatalf("expected 8 lines, got %d:\n%s", lines, view)
	}
}

func TestSelectedCalendarDayCreatesVirtualDay(t *testing.T) {
	monthTime := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	month := &viewmodel.ParsedCollection{
		ID:     "January 2024",
		Name:   "January 2024",
		Type:   collection.TypeDaily,
		Exists: true,
		Month:  monthTime,
	}

	model := NewModel([]*viewmodel.ParsedCollection{month})
	model.SetNow(monthTime)
	if cal := model.ensureCalendar(month); cal != nil {
		cal.SetSelected(5)
	}

	day, exists := model.selectedCalendarDay(month)
	if day == nil {
		t.Fatalf("expected virtual day to be returned")
	}
	if exists {
		t.Fatalf("expected virtual day to be marked missing")
	}
	if day.Name != "January 5, 2024" {
		t.Fatalf("expected day name January 5, 2024, got %q", day.Name)
	}
}

func TestCalendarChildrenSortedByDay(t *testing.T) {
	monthTime := time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2024, time.March, 2, 0, 0, 0, 0, time.UTC)
	day5 := time.Date(2024, time.March, 5, 0, 0, 0, 0, time.UTC)
	day10 := time.Date(2024, time.March, 10, 0, 0, 0, 0, time.UTC)

	month := &viewmodel.ParsedCollection{
		ID:     "March 2024",
		Name:   "March 2024",
		Type:   collection.TypeDaily,
		Exists: true,
		Month:  monthTime,
		Days: []viewmodel.DaySummary{
			{ID: "March 2024/March 10, 2024", Name: "March 10, 2024", Date: day10},
			{ID: "March 2024/March 2, 2024", Name: "March 2, 2024", Date: day2},
		},
		Children: []*viewmodel.ParsedCollection{
			{ID: "March 2024/March 5, 2024", Name: "March 5, 2024", Day: day5},
		},
	}

	model := NewModel([]*viewmodel.ParsedCollection{month})
	model.calendarExtras = map[string]map[int]indexview.CollectionItem{
		month.ID: {
			3: {Name: "March 3, 2024", Resolved: "March 2024/March 3, 2024"},
		},
	}

	children := model.calendarChildren(month)
	if len(children) != 4 {
		t.Fatalf("expected 4 calendar children, got %d", len(children))
	}
	got := []string{children[0].Name, children[1].Name, children[2].Name, children[3].Name}
	want := []string{"March 2, 2024", "March 3, 2024", "March 5, 2024", "March 10, 2024"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected day %d to be %q, got %q", i, want[i], got[i])
		}
	}
}

func TestHighlightAndSelectMessagesForDayAndGeneric(t *testing.T) {
	monthTime := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	month := &viewmodel.ParsedCollection{
		ID:     "January 2024",
		Name:   "January 2024",
		Type:   collection.TypeDaily,
		Exists: true,
		Month:  monthTime,
	}
	generic := &viewmodel.ParsedCollection{
		ID:     "Inbox",
		Name:   "Inbox",
		Type:   collection.TypeGeneric,
		Exists: true,
	}

	model := NewModel([]*viewmodel.ParsedCollection{month, generic})
	model.SetNow(monthTime)
	_ = model.Focus()
	if cal := model.ensureCalendar(month); cal != nil {
		cal.SetSelected(3)
	}

	model.list.Select(0)
	if cmd := model.highlightCmd(); cmd != nil {
		msg := cmd().(events.CollectionHighlightMsg)
		if msg.RowKind != "day" {
			t.Fatalf("expected day highlight, got %q", msg.RowKind)
		}
	}

	item, ok := model.selectedNavItem()
	if !ok {
		t.Fatalf("expected selected nav item")
	}
	target, kind, exists := model.selectionTarget(item)
	selectMsg := selectCmd(model.id, target, kind, exists)().(events.CollectionSelectMsg)
	if selectMsg.RowKind != "day" {
		t.Fatalf("expected day select, got %q", selectMsg.RowKind)
	}

	model.list.Select(1)
	item, ok = model.selectedNavItem()
	if !ok {
		t.Fatalf("expected selected nav item")
	}
	_, kind, _ = model.selectionTarget(item)
	if kind.String() != "generic" {
		t.Fatalf("expected generic kind, got %q", kind.String())
	}
}
