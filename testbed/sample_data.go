package main

import (
	"time"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
)

func sampleDetailSections() []collectiondetail.Section {
	now := time.Now()
	return []collectiondetail.Section{
		{
			ID:    "Inbox",
			Title: "Inbox",
			Bullets: []collectiondetail.Bullet{
				{
					ID:        "0",
					Label:     "FIRST",
					Bullet:    glyph.Task,
					Signifier: glyph.Priority,
					Created:   now.Add(-2 * time.Hour),
				},
				{
					ID:        "1",
					Label:     "Draft release notes",
					Bullet:    glyph.Task,
					Signifier: glyph.Priority,
					Created:   now.Add(-2 * time.Hour),
				},
				{
					ID:        "2",
					Label:     "Review pull requests",
					Bullet:    glyph.Event,
					Signifier: glyph.None,
					Created:   now.Add(-90 * time.Minute),
					Children: []collectiondetail.Bullet{
						{
							ID:        "2-1",
							Label:     "UI polish PR",
							Bullet:    glyph.Task,
							Signifier: glyph.Priority,
							Created:   now.Add(-80 * time.Minute),
						},
						{
							ID:      "2-2",
							Label:   "Write an extra long review comment about the storage refactor PR so we can verify wrapping works for nested subtasks that exceed the available width in the detail pane",
							Bullet:  glyph.Task,
							Created: now.Add(-70 * time.Minute),
						},
					},
				},
				{
					ID:        "3",
					Label:     "Email Alex about the demo",
					Bullet:    glyph.Note,
					Signifier: glyph.Inspiration,
					Created:   now.Add(-30 * time.Minute),
				},
				{
					ID:        "8",
					Label:     "Write a really long description for this task so we can verify wrapping behaves correctly when the line length greatly exceeds the available width in the detail pane",
					Bullet:    glyph.Task,
					Signifier: glyph.None,
					Created:   now,
				},
				{
					ID:        "9",
					Label:     "Archive old OKR doc",
					Bullet:    glyph.MovedCollection,
					Signifier: glyph.None,
					Created:   now.Add(-15 * time.Minute),
				},
			},
		},
		{
			ID:       "October 2025",
			Title:    "October 2025",
			Subtitle: "Daily overview",
			Bullets: []collectiondetail.Bullet{
				{ID: "4", Label: "Standup", Bullet: glyph.Event, Created: now.Add(-4 * time.Hour)},
				{ID: "5", Label: "Ship calendar refactor", Bullet: glyph.Task, Signifier: glyph.Investigation, Created: now.Add(-3 * time.Hour)},
				{ID: "6", Label: "Plan weekend hike", Bullet: glyph.Note, Created: now.Add(-2 * time.Hour)},
				{
					ID:        "10",
					Label:     "Send launch email to list (done!)",
					Bullet:    glyph.Completed,
					Signifier: glyph.None,
					Created:   now.Add(-1 * time.Hour),
				},
				{
					ID:        "11",
					Label:     "Purge deprecated scripts (no longer needed)",
					Bullet:    glyph.Irrelevant,
					Signifier: glyph.None,
					Created:   now.Add(-30 * time.Minute),
				},
			},
		},
		{
			ID:    "Future",
			Title: "Future",
			Bullets: []collectiondetail.Bullet{
				{ID: "7", Label: "Book flights to NYC", Bullet: glyph.Task, Created: now.Add(24 * time.Hour)},
				{ID: "12", Label: "Reschedule dentist appointment", Bullet: glyph.MovedFuture, Created: now.Add(48 * time.Hour)},
			},
		},
		{
			ID:    "Projects",
			Title: "Projects",
			Bullets: []collectiondetail.Bullet{
				{ID: "13", Label: "Side Quest backlog grooming", Bullet: glyph.Task, Created: now.Add(-6 * time.Hour)},
				{ID: "14", Label: "Metrics dashboard polish01", Bullet: glyph.Task, Created: now.Add(-5 * time.Hour)},
				{ID: "15", Label: "Metrics dashboard polish02", Bullet: glyph.Task, Created: now.Add(-5 * time.Hour)},
				{ID: "16", Label: "Metrics dashboard polish03", Bullet: glyph.Task, Created: now.Add(-5 * time.Hour)},
				{ID: "17", Label: "Metrics dashboard polish04", Bullet: glyph.Task, Created: now.Add(-5 * time.Hour)},
				{ID: "18", Label: "Metrics dashboard polish05", Bullet: glyph.Task, Created: now.Add(-5 * time.Hour)},
				{ID: "19", Label: "Metrics dashboard polish06", Bullet: glyph.Task, Created: now.Add(-5 * time.Hour)},
				{ID: "20", Label: "Metrics dashboard polish07", Bullet: glyph.Task, Created: now.Add(-5 * time.Hour)},
				{ID: "21", Label: "Metrics dashboard polish08", Bullet: glyph.Task, Created: now.Add(-5 * time.Hour)},
				{ID: "22", Label: "Metrics dashboard polish09", Bullet: glyph.Task, Created: now.Add(-5 * time.Hour)},
				{ID: "23", Label: "Metrics dashboard polish10", Bullet: glyph.Task, Created: now.Add(-5 * time.Hour)},
				{ID: "24", Label: "Metrics dashboard polish11", Bullet: glyph.Task, Created: now.Add(-5 * time.Hour)},
				{ID: "25", Label: "Metrics dashboard polish12", Bullet: glyph.Task, Created: now.Add(-5 * time.Hour)},
				{ID: "26", Label: "Metrics dashboard polish13", Bullet: glyph.Task, Created: now.Add(-5 * time.Hour)},
				{ID: "27", Label: "Metrics dashboard polish14", Bullet: glyph.Task, Created: now.Add(-5 * time.Hour)},
				{ID: "28", Label: "Metrics dashboard polish15", Bullet: glyph.Task, Created: now.Add(-5 * time.Hour)},
				{ID: "29", Label: "LAST", Bullet: glyph.Task, Created: now.Add(-5 * time.Hour)},
			},
		},
	}
}

func sampleCollectionData() ([]collection.Meta, map[string]int) {
	metas := []collection.Meta{
		{Name: "Inbox", Type: collection.TypeGeneric},
		{Name: "Future", Type: collection.TypeMonthly},
		{Name: "Future/December 2025", Type: collection.TypeGeneric},
		{Name: "October 2025", Type: collection.TypeDaily},
		{Name: "October 2025/October 5, 2025", Type: collection.TypeGeneric},
		{Name: "October 2025/October 12, 2025", Type: collection.TypeGeneric},
		{Name: "October 2025/October 22, 2025", Type: collection.TypeGeneric},
		{Name: "November 2025", Type: collection.TypeDaily},
		{Name: "November 2025/November 22, 2025", Type: collection.TypeGeneric},
		{Name: "Projects", Type: collection.TypeGeneric},
		{Name: "Projects/Side Quest", Type: collection.TypeGeneric},
		{Name: "Metrics", Type: collection.TypeTracking},
	}
	priorities := map[string]int{
		"Inbox":  0,
		"Future": 10,
	}
	return metas, priorities
}
