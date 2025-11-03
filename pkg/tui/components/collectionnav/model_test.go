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
	if trimmed := strings.TrimRight(view, "\n "); trimmed != view {
		t.Fatalf("view contains trailing padding:\n%s", view)
	}
}
