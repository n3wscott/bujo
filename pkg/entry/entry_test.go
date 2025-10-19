package entry

import (
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
