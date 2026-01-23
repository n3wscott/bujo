package cache

import (
	"testing"
	"time"

	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
)

func TestBuildBulletsOrdersRootsAndChildren(t *testing.T) {
	t0 := time.Date(2025, 10, 1, 9, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	t2 := t1.Add(time.Hour)
	t3 := t2.Add(time.Hour)

	entries := []*entry.Entry{
		newEntry("child-2", "a", t3),
		newEntry("b", "", t0),
		newEntry("child-1", "a", t2),
		newEntry("a", "", t1),
	}

	bullets := buildBullets(entries)
	if len(bullets) != 2 {
		t.Fatalf("expected 2 root bullets, got %d", len(bullets))
	}
	if bullets[0].ID != "b" {
		t.Fatalf("expected first root to be b, got %q", bullets[0].ID)
	}
	if bullets[1].ID != "a" {
		t.Fatalf("expected second root to be a, got %q", bullets[1].ID)
	}
	if len(bullets[1].Children) != 2 {
		t.Fatalf("expected 2 children for a, got %d", len(bullets[1].Children))
	}
	if bullets[1].Children[0].ID != "child-1" {
		t.Fatalf("expected first child to be child-1, got %q", bullets[1].Children[0].ID)
	}
	if bullets[1].Children[1].ID != "child-2" {
		t.Fatalf("expected second child to be child-2, got %q", bullets[1].Children[1].ID)
	}
}

func TestBuildBulletsOrphansBecomeRoots(t *testing.T) {
	t0 := time.Date(2025, 11, 5, 9, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)

	entries := []*entry.Entry{
		newEntry("orphan", "missing", t1),
		newEntry("root", "", t0),
	}

	bullets := buildBullets(entries)
	if len(bullets) != 2 {
		t.Fatalf("expected 2 root bullets, got %d", len(bullets))
	}
	if bullets[0].ID != "root" {
		t.Fatalf("expected first root to be root, got %q", bullets[0].ID)
	}
	if bullets[1].ID != "orphan" {
		t.Fatalf("expected orphan to remain a root, got %q", bullets[1].ID)
	}
}

func TestBuildBulletsHandlesCycles(t *testing.T) {
	t0 := time.Date(2025, 12, 1, 9, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)

	entries := []*entry.Entry{
		newEntry("a", "b", t0),
		newEntry("b", "a", t1),
	}

	bullets := buildBullets(entries)
	if len(bullets) != 2 {
		t.Fatalf("expected 2 root bullets for cycle, got %d", len(bullets))
	}
}

func TestDedupeBulletsMergesByID(t *testing.T) {
	bullets := []collectiondetail.Bullet{
		{ID: "dup", Label: "first", Bullet: glyph.Note},
		{ID: "dup", Label: "second", Signifier: glyph.Priority},
	}

	result := dedupeBullets(bullets)
	if len(result) != 1 {
		t.Fatalf("expected 1 bullet after dedupe, got %d", len(result))
	}
	if result[0].Label != "second" {
		t.Fatalf("expected merged label to prefer updated value, got %q", result[0].Label)
	}
	if result[0].Signifier != glyph.Priority {
		t.Fatalf("expected merged signifier to be preserved, got %q", result[0].Signifier)
	}
}

func newEntry(id, parent string, created time.Time) *entry.Entry {
	return &entry.Entry{
		ID:       id,
		Bullet:   glyph.Task,
		Created:  entry.Timestamp{Time: created},
		Message:  id,
		ParentID: parent,
	}
}
