package viewmodel

import (
	"testing"
	"time"

	"tableflip.dev/bujo/pkg/collection"
)

func TestBuildTreeHierarchy(t *testing.T) {
	metas := []collection.Meta{
		{Name: "Future", Type: collection.TypeMonthly},
		{Name: "Future/October 2025", Type: collection.TypeDaily},
		{Name: "Future/October 2025/October 11, 2025", Type: collection.TypeGeneric},
		{Name: "Inbox", Type: collection.TypeGeneric},
	}

	roots := BuildTree(metas)
	if len(roots) != 2 {
		t.Fatalf("expected 2 root collections, got %d", len(roots))
	}

	var future *ParsedCollection
	for _, root := range roots {
		if root.ID == "Future" {
			future = root
			break
		}
	}
	if future == nil {
		t.Fatalf("Future root not found")
	}
	if len(future.Children) != 1 {
		t.Fatalf("Future should have 1 child, got %d", len(future.Children))
	}

	month := future.Children[0]
	if month.ID != "Future/October 2025" {
		t.Fatalf("unexpected child ID: %s", month.ID)
	}
	if month.Month.IsZero() {
		t.Fatalf("expected month metadata parsed")
	}
	if month.Month.Month() != time.October || month.Month.Year() != 2025 {
		t.Fatalf("unexpected month parsed: %v", month.Month)
	}
	if len(month.Children) != 1 {
		t.Fatalf("expected day child, got %d", len(month.Children))
	}
	if len(month.Days) != 1 {
		t.Fatalf("expected one day summary, got %d", len(month.Days))
	}
	day := month.Days[0]
	if day.Name != "October 11, 2025" {
		t.Fatalf("unexpected day summary name: %s", day.Name)
	}
	if day.Date.Day() != 11 {
		t.Fatalf("unexpected day parsed: %v", day.Date)
	}
}

func TestBuildTreeWithCustomPriorities(t *testing.T) {
	metas := []collection.Meta{
		{Name: "Zeta"},
		{Name: "Inbox"},
	}
	roots := BuildTree(metas, WithPriorities(map[string]int{
		"Inbox": 0,
	}))
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots")
	}
	if roots[0].ID != "Inbox" {
		t.Fatalf("expected Inbox to be first root, got %s", roots[0].ID)
	}
	if roots[0].Priority != 0 {
		t.Fatalf("expected custom priority applied, got %d", roots[0].Priority)
	}
}
