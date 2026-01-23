package collectionnav

import (
	"strings"
	"testing"
	"time"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/collection/viewmodel"
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
