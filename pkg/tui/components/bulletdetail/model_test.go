package bulletdetail

import (
	"strings"
	"testing"
	"time"

	"github.com/muesli/reflow/ansi"

	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
)

func stripANSIString(s string) string {
	var b strings.Builder
	ansiSeq := false
	for _, r := range s {
		if r == ansi.Marker {
			ansiSeq = true
			continue
		}
		if ansiSeq {
			if ansi.IsTerminator(r) {
				ansiSeq = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func TestViewRendersLabelsAndDependsOn(t *testing.T) {
	model := New(
		"Saturday, January 24, 2026",
		"something",
		"January 2026/January 24, 2026",
		"January 2026/January 24, 2026",
	)
	model.SetEntry(&entry.Entry{
		Bullet:    glyph.Task,
		Signifier: glyph.None,
		Created:   entry.Timestamp{Time: time.Date(2026, 1, 24, 7, 58, 0, 0, time.UTC)},
		Message:   "something",
		Labels:    []string{"owner:codex", "area:api"},
		DependsOn: []string{"parent456", "parent789"},
		History: []entry.HistoryRecord{
			{
				Timestamp: entry.Timestamp{Time: time.Date(2026, 1, 24, 7, 58, 0, 0, time.UTC)},
				Action:    entry.HistoryActionAdded,
				To:        "January 2026/January 24, 2026",
			},
		},
	})

	view, _ := model.View()
	plain := stripANSIString(view)
	if !strings.Contains(plain, "Labels: owner:codex, area:api") {
		t.Fatalf("expected labels in detail view, got:\n%s", plain)
	}
	if !strings.Contains(plain, "Depends On: parent456, parent789") {
		t.Fatalf("expected depends_on in detail view, got:\n%s", plain)
	}
}

func TestViewShowsEmptyLabelsAndDependsOn(t *testing.T) {
	model := New("Collection", "bullet", "Inbox", "Inbox")
	model.SetEntry(&entry.Entry{
		Bullet:    glyph.Task,
		Signifier: glyph.None,
		Message:   "thing",
	})

	view, _ := model.View()
	plain := stripANSIString(view)
	if !strings.Contains(plain, "Labels: (none)") {
		t.Fatalf("expected empty labels in detail view, got:\n%s", plain)
	}
	if !strings.Contains(plain, "Depends On: (none)") {
		t.Fatalf("expected empty depends_on in detail view, got:\n%s", plain)
	}
}
