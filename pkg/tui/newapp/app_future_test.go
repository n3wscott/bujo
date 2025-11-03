package newapp

import (
	"testing"
	"time"

	"tableflip.dev/bujo/pkg/collection"
)

func TestFutureCollectionsFromMetas(t *testing.T) {
	now := time.Date(2024, time.July, 10, 12, 0, 0, 0, time.UTC)
	metas := []collection.Meta{
		{Name: "Future", Type: collection.TypeMonthly},
		{Name: "Future/October 2024", Type: collection.TypeDaily},
		{Name: "Future/December 2024", Type: collection.TypeDaily},
	}

	tree := futureCollectionsFromMetas(metas, now)
	if len(tree) != 1 {
		t.Fatalf("expected one root, got %d", len(tree))
	}
	root := tree[0]
	if root.ID != "Future" {
		t.Fatalf("expected root ID Future, got %q", root.ID)
	}
	if !root.Exists {
		t.Fatalf("expected Future root to exist")
	}
	if root.Type != collection.TypeMonthly {
		t.Fatalf("expected Future type monthly, got %s", root.Type)
	}

	children := root.Children
	if len(children) != 12 {
		t.Fatalf("expected 12 month children, got %d", len(children))
	}
	if children[0].Name != "August 2024" {
		t.Fatalf("expected first child to be August 2024, got %q", children[0].Name)
	}
	if children[0].Exists {
		t.Fatalf("expected August 2024 to be marked missing")
	}
	if children[0].Type != collection.TypeGeneric {
		t.Fatalf("expected month type generic, got %s", children[0].Type)
	}
	if children[len(children)-1].Name != "July 2025" {
		t.Fatalf("expected last child to be July 2025, got %q", children[len(children)-1].Name)
	}

	checkExists := func(name string, want bool) {
		t.Helper()
		for _, child := range children {
			if child.Name == name {
				if child.Exists != want {
					t.Fatalf("expected %s exists=%t, got %t", name, want, child.Exists)
				}
				return
			}
		}
		t.Fatalf("month %s not found in children", name)
	}

	checkExists("October 2024", true)
	checkExists("December 2024", true)
	checkExists("September 2024", false)
	checkExists("June 2025", false)
}
