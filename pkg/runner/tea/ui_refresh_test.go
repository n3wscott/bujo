package teaui

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/v2/list"

	"tableflip.dev/bujo/pkg/runner/tea/internal/indexview"
)

// This regression test ensures refreshCalendarMonth keeps header and weeks stable.
func TestRefreshCalendarMonthRebuildsRowsWithoutDuplication(t *testing.T) {
	now := time.Date(2025, time.November, 5, 0, 0, 0, 0, time.UTC)
	model := New(nil)
	model.indexState.Selection["November 2025"] = 1

	month := "November 2025"
	monthTime := time.Date(2025, time.November, 1, 0, 0, 0, 0, time.UTC)

	header, weeks := indexview.RenderCalendarRows(month, monthTime, nil, 1, now, indexview.DefaultCalendarOptions())
	if header == nil {
		t.Fatalf("expected header")
	}
	if len(weeks) == 0 {
		t.Fatalf("expected at least one week row")
	}
	t.Logf("initial weeks: %d", len(weeks))
	for _, w := range weeks {
		if w == nil {
			t.Fatalf("week should not be nil")
		}
	}

	items := []list.Item{
		indexview.CollectionItem{Name: "Future", Resolved: "Future"},
		indexview.CollectionItem{Name: month, Resolved: month},
		header,
	}
	for _, w := range weeks {
		w.RowIndex = len(items)
		items = append(items, w)
	}

	state := &indexview.MonthState{
		Month:     month,
		MonthTime: monthTime,
		Children:  nil,
		HeaderIdx: 2,
		Weeks:     weeks,
	}

	model.indexState.Months[month] = state
	model.colList.SetItems(items)

	for _, day := range []int{1, 8, 15, 22, 29, 30} {
		model.indexState.Selection[month] = day
		model.refreshCalendarMonth(month)

		got := model.colList.Items()
		if len(got) != 3+len(weeks) {
			t.Fatalf("day %d: expected %d items, got %d", day, 3+len(weeks), len(got))
		}
		hdr, ok := got[2].(*indexview.CalendarHeaderItem)
		if !ok {
			t.Fatalf("day %d: expected header at position 2, got %T", day, got[2])
		}
		if hdr.Month != month {
			t.Fatalf("day %d: header month mismatch %q", day, hdr.Month)
		}
		for i := 0; i < len(weeks); i++ {
			item, ok := got[3+i].(*indexview.CalendarRowItem)
			if !ok {
				t.Fatalf("day %d: expected calendar row at %d, got %T", day, 3+i, got[3+i])
			}
			if item == nil || item.Month != month {
				t.Fatalf("day %d: unexpected calendar row data at %d: %#v", day, 3+i, item)
			}
		}
	}
}

func TestBuildCollectionItemsGrouping(t *testing.T) {
	model := New(nil)
	now := time.Date(2025, time.November, 5, 0, 0, 0, 0, time.UTC)
	cols := []string{
		"October 2025",
		"October 2025/October 11, 2025",
		"September 2025",
		"Projects",
		"Projects/Alpha",
	}

	items := model.buildCollectionItems(cols, "", now)
	if len(items) == 0 {
		t.Fatalf("expected items")
	}

	if ci, ok := items[0].(indexview.CollectionItem); !ok || ci.Name != todayMetaName {
		t.Fatalf("expected Today meta item first, got %#v", items[0])
	}

	monthOrder := make([]string, 0)
	otherOrder := make([]string, 0)

	for _, it := range items {
		switch v := it.(type) {
		case indexview.CollectionItem:
			if v.Indent {
				continue
			}
			if v.Name == todayMetaName {
				continue
			}
			if _, ok := indexview.ParseMonth(v.Name); ok {
				monthOrder = append(monthOrder, v.Name)
			} else {
				otherOrder = append(otherOrder, v.Name)
			}
		}
	}

	expectedMonths := []string{"November 2025", "October 2025", "September 2025"}
	if len(monthOrder) < len(expectedMonths) {
		t.Fatalf("expected at least %d months, got %v", len(expectedMonths), monthOrder)
	}
	for i, name := range expectedMonths {
		if i >= len(monthOrder) {
			break
		}
		if monthOrder[i] != name {
			t.Fatalf("month order mismatch at %d: want %s, got %s", i, name, monthOrder[i])
		}
	}

	if expanded := model.indexState.Fold["November 2025"]; expanded {
		t.Fatalf("expected current month to be expanded by default")
	}
	if collapsed := model.indexState.Fold["October 2025"]; !collapsed {
		t.Fatalf("expected non-current month to be collapsed by default")
	}
}
