package entry

import (
	"reflect"
	"testing"
	"time"
)

func TestLastCompletionTime(t *testing.T) {
	now := time.Now()
	e := &Entry{
		History: []HistoryRecord{
			{
				Timestamp: Timestamp{Time: now.Add(-48 * time.Hour)},
				Action:    HistoryActionAdded,
				To:        "Inbox",
			},
			{
				Timestamp: Timestamp{Time: now.Add(-24 * time.Hour)},
				Action:    HistoryActionCompleted,
				To:        "Today",
			},
			{
				Timestamp: Timestamp{Time: now.Add(-12 * time.Hour)},
				Action:    HistoryActionCompleted,
				To:        "Today",
			},
		},
	}

	ts, ok := e.LastCompletionTime()
	if !ok {
		t.Fatalf("expected completion timestamp")
	}
	if !ts.Equal(now.Add(-12 * time.Hour)) {
		t.Fatalf("expected latest completion, got %v", ts)
	}
}

func TestLastCompletionTimeNone(t *testing.T) {
	e := &Entry{
		History: []HistoryRecord{
			{
				Timestamp: Timestamp{Time: time.Now()},
				Action:    HistoryActionAdded,
				To:        "Inbox",
			},
		},
	}
	if _, ok := e.LastCompletionTime(); ok {
		t.Fatalf("expected no completion timestamp")
	}
}

func TestNormalizeLabels(t *testing.T) {
	got := NormalizeLabels([]string{"Owner:Codex", " owner:codex ", "", "Area:API"})
	want := []string{"area:api", "owner:codex"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected labels: got=%v want=%v", got, want)
	}
}

func TestEntryLabelMutations(t *testing.T) {
	e := &Entry{}
	e.SetLabels([]string{"owner:codex", "area:api"})
	e.AddLabels([]string{"state:open", "owner:codex"})
	if !reflect.DeepEqual(e.Labels, []string{"area:api", "owner:codex", "state:open"}) {
		t.Fatalf("unexpected labels after add: %v", e.Labels)
	}
	e.RemoveLabels([]string{"owner:codex"})
	if !reflect.DeepEqual(e.Labels, []string{"area:api", "state:open"}) {
		t.Fatalf("unexpected labels after remove: %v", e.Labels)
	}
	e.SetLabels(nil)
	if e.Labels != nil {
		t.Fatalf("expected nil labels after set nil, got %v", e.Labels)
	}
}

func TestNormalizeDependsOnIDs(t *testing.T) {
	got := NormalizeDependsOnIDs([]string{" dep-b ", "dep-a", "dep-b", ""})
	want := []string{"dep-a", "dep-b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected depends_on ids: got=%v want=%v", got, want)
	}
}

func TestEntryDependsOnMutations(t *testing.T) {
	e := &Entry{}
	e.SetDependsOn([]string{"dep-b", "dep-a"})
	e.AddDependsOn([]string{"dep-c", "dep-a"})
	if !reflect.DeepEqual(e.DependsOn, []string{"dep-a", "dep-b", "dep-c"}) {
		t.Fatalf("unexpected depends_on after add: %v", e.DependsOn)
	}
	e.RemoveDependsOn([]string{"dep-b"})
	if !reflect.DeepEqual(e.DependsOn, []string{"dep-a", "dep-c"}) {
		t.Fatalf("unexpected depends_on after remove: %v", e.DependsOn)
	}
	e.SetDependsOn(nil)
	if e.DependsOn != nil {
		t.Fatalf("expected nil depends_on after set nil, got %v", e.DependsOn)
	}
}
