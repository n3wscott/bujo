package index

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/v2/list"

	"tableflip.dev/bujo/pkg/collection"
)

func TestBuildItemsOrdering(t *testing.T) {
	state := NewState()
	now := time.Date(2024, time.January, 15, 0, 0, 0, 0, time.UTC)
	metas := []collection.Meta{
		{Name: "Future", Type: collection.TypeMonthly},
		{Name: "January 2024", Type: collection.TypeDaily},
		{Name: "December 2023", Type: collection.TypeDaily},
		{Name: "Inbox", Type: collection.TypeGeneric},
		{Name: "Track", Type: collection.TypeTracking},
	}

	items := BuildItems(state, metas, "", now)
	order := collectionItemNames(items)

	want := []string{"Future", "January 2024", "December 2023", "Inbox", "Tracking", "Track"}
	if len(order) < len(want) {
		t.Fatalf("expected at least %d collection items, got %d", len(want), len(order))
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("expected item %d to be %q, got %q", i, want[i], order[i])
		}
	}
}

func TestDefaultSelectedDay(t *testing.T) {
	now := time.Date(2024, time.January, 10, 0, 0, 0, 0, time.UTC)
	month := "January 2024"
	monthTime := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)

	selected := DefaultSelectedDay(month, monthTime, nil, "January 2024/January 5, 2024", now)
	if selected != 5 {
		t.Fatalf("expected currentResolved to win, got %d", selected)
	}

	selected = DefaultSelectedDay(month, monthTime, nil, "", now)
	if selected != 10 {
		t.Fatalf("expected current month to select today, got %d", selected)
	}

	otherMonth := "February 2024"
	otherTime := time.Date(2024, time.February, 1, 0, 0, 0, 0, time.UTC)
	children := []CollectionItem{{Name: "February 2, 2024", Resolved: "February 2024/February 2, 2024"}}
	selected = DefaultSelectedDay(otherMonth, otherTime, children, "", now)
	if selected != 2 {
		t.Fatalf("expected first child day to be selected, got %d", selected)
	}
}

func TestRenderCalendarRowsRowCount(t *testing.T) {
	monthTime := time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)
	header, rows := RenderCalendarRows("March 2024", monthTime, nil, 0, monthTime, DefaultCalendarOptions())
	if header == nil {
		t.Fatalf("expected header to be returned")
	}
	if len(rows) != 6 {
		t.Fatalf("expected 6 rows for March 2024, got %d", len(rows))
	}
}

func collectionItemNames(items []list.Item) []string {
	var names []string
	for _, item := range items {
		if col, ok := item.(CollectionItem); ok {
			names = append(names, col.Name)
		}
	}
	return names
}
