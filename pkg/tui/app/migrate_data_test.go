package app

import (
	"strings"
	"testing"
	"time"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/collection"
	viewmodel "tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
)

func TestBuildMigrationDataSections(t *testing.T) {
	now := time.Date(2024, time.March, 10, 8, 0, 0, 0, time.UTC)
	metas := []collection.Meta{
		{Name: "Inbox", Type: collection.TypeGeneric},
		{Name: "Future", Type: collection.TypeMonthly},
		{Name: "March 2024", Type: collection.TypeDaily},
		{Name: "March 2024/March 8, 2024", Type: collection.TypeGeneric},
	}
	parsed := viewmodel.BuildTree(metas)

	parent := &entry.Entry{
		ID:         "parent",
		Collection: "Inbox",
		Bullet:     glyph.Task,
		Message:    "Parent Task",
		Created:    entry.Timestamp{Time: now.Add(-48 * time.Hour)},
	}
	candidates := []app.MigrationCandidate{
		{
			Entry: &entry.Entry{
				ID:         "task-1",
				Collection: "Inbox",
				Bullet:     glyph.Task,
				Message:    "Primary task",
				Created:    entry.Timestamp{Time: now.Add(-2 * time.Hour)},
			},
			Parent:      parent,
			LastTouched: now.Add(-2 * time.Hour),
		},
		{
			Entry: &entry.Entry{
				ID:         "future-root",
				Collection: "Future",
				Bullet:     glyph.Task,
				Message:    "Future item",
				Created:    entry.Timestamp{Time: now.Add(-7 * 24 * time.Hour)},
			},
			LastTouched: now.Add(-7 * 24 * time.Hour),
		},
		{
			Entry: &entry.Entry{
				ID:         "day-task",
				Collection: "March 2024/March 8, 2024",
				Bullet:     glyph.Task,
				Message:    "Daily task",
				Created:    entry.Timestamp{Time: now.Add(-24 * time.Hour)},
			},
			LastTouched: now.Add(-24 * time.Hour),
		},
	}

	data := buildMigrationData(now, candidates, parsed)
	sections := data.Sections()
	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(sections))
	}
	if sections[0].ID != "Inbox" {
		t.Fatalf("expected first section to be Inbox, got %q", sections[0].ID)
	}
	if len(sections[0].Bullets) != 1 {
		t.Fatalf("expected Inbox section to have 1 bullet, got %d", len(sections[0].Bullets))
	}
	note := sections[0].Bullets[0].Note
	if note == "" || !strings.Contains(note, "parent: Parent Task") {
		t.Fatalf("expected note to mention parent label, got %q", note)
	}

	// Ensure removal updates sections and empties when all items removed.
	if removed := data.Remove("task-1"); !removed {
		t.Fatalf("expected removal of task-1 to succeed")
	}
	if data.IsEmpty() {
		t.Fatalf("data should not be empty after removing one bullet")
	}
	data.Remove("future-root")
	data.Remove("day-task")
	if !data.IsEmpty() {
		t.Fatalf("expected data to be empty after removing all bullets")
	}
}
