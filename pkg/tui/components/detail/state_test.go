package detail

import (
	"fmt"
	"strings"
	"testing"

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

func makeEntries(count int) []*entry.Entry {
	entries := make([]*entry.Entry, count)
	for i := 0; i < count; i++ {
		e := &entry.Entry{
			ID:      formatID(i),
			Message: "item",
			Bullet:  glyph.Task,
		}
		e.EnsureHistorySeed()
		entries[i] = e
	}
	return entries
}

func formatID(i int) string {
	return fmt.Sprintf("%03d", i)
}

func TestSetSectionsPreservesScrollOffset(t *testing.T) {
	entries := makeEntries(10)
	sections := []Section{{
		CollectionID:   "A",
		CollectionName: "A",
		Entries:        entries,
	}}

	state := NewState()
	state.SetSections(sections)
	state.SetActive("A", entries[0].ID)
	state.Viewport(4)

	for i := 0; i < 6; i++ {
		state.MoveEntry(1)
	}
	state.Viewport(4)

	if state.scrollOffset == 0 {
		t.Fatalf("expected scroll offset to advance after moving, got 0")
	}
	before := state.scrollOffset
	currentID := state.ActiveEntryID()

	state.SetSections(sections)
	state.SetActive("A", currentID)
	state.Viewport(4)

	if state.scrollOffset != before {
		t.Fatalf("expected scroll offset %d after reload, got %d", before, state.scrollOffset)
	}

	state.MoveEntry(-1)
	state.Viewport(4)
	if state.scrollOffset > before {
		t.Fatalf("scroll offset increased after moving up: before %d, after %d", before, state.scrollOffset)
	}
}

func TestRevealCollectionPrefersFullView(t *testing.T) {
	secA := Section{CollectionID: "A", CollectionName: "A", Entries: makeEntries(2)}
	secB := Section{CollectionID: "B", CollectionName: "B", Entries: makeEntries(1)}
	secC := Section{CollectionID: "C", CollectionName: "C", Entries: makeEntries(6)}
	state := NewState()
	state.SetSections([]Section{secA, secB, secC})
	state.SetActive("C", "000")
	state.Viewport(6)

	state.RevealCollection("B", true, 6)
	if state.scrollOffset != 1 {
		t.Fatalf("expected scroll offset 1 to show entire section B, got %d", state.scrollOffset)
	}

	state.RevealCollection("C", true, 6)
	if state.scrollOffset != 7 {
		t.Fatalf("expected scroll offset 7 to pin header for large section, got %d", state.scrollOffset)
	}
}

func TestEnsureScrollVisibleKeepsCursorVisible(t *testing.T) {
	sections := []Section{
		{
			CollectionID:   "A",
			CollectionName: "A",
			Entries: []*entry.Entry{
				{
					ID:      "A-0",
					Message: "first line\nsecond line",
					Bullet:  glyph.Task,
				},
			},
		},
		{
			CollectionID:   "B",
			CollectionName: "B",
			Entries: []*entry.Entry{
				{
					ID:      "B-0",
					Message: "item",
					Bullet:  glyph.Task,
				},
			},
		},
		{
			CollectionID:   "C",
			CollectionName: "C",
			Entries: []*entry.Entry{
				{
					ID:      "C-0",
					Message: "target",
					Bullet:  glyph.Task,
				},
			},
		},
	}
	for _, sec := range sections {
		for _, e := range sec.Entries {
			e.EnsureHistorySeed()
		}
	}

	state := NewState()
	state.SetSections(sections)
	state.SetActive("C", "C-0")

	view, _ := state.Viewport(4)
	plain := stripANSIString(view)
	if !strings.Contains(plain, "→") {
		t.Fatalf("expected caret to be visible in viewport, got:\n%s", plain)
	}
}

func TestEnsureScrollVisibleAccountsForSectionSpacing(t *testing.T) {
	longLine := "Write an extra long review comment about the storage refactor PR so we can verify wrapping works correctly."
	wrapEntries := []*entry.Entry{
		{ID: "wrap-1", Message: longLine, Bullet: glyph.Task},
		{ID: "wrap-2", Message: "! Email Alex about the demo", Bullet: glyph.Note},
		{ID: "wrap-3", Message: longLine + " Even more wrapping to ensure the section pushes the next header down.", Bullet: glyph.Task},
	}
	listEntries := make([]*entry.Entry, 0, 12)
	for i := 0; i < 12; i++ {
		listEntries = append(listEntries, &entry.Entry{
			ID:      fmt.Sprintf("list-%02d", i),
			Message: fmt.Sprintf("Metrics dashboard polish%02d", i),
			Bullet:  glyph.Task,
		})
	}
	for _, e := range append(wrapEntries, listEntries...) {
		e.EnsureHistorySeed()
	}

	sections := []Section{
		{
			CollectionID:   "Inbox",
			CollectionName: "Inbox",
			Entries:        wrapEntries,
		},
		{
			CollectionID:   "Projects",
			CollectionName: "Projects",
			Entries:        listEntries,
		},
	}

	state := NewState()
	state.SetWrapWidth(40)
	state.SetSections(sections)
	state.SetActive("Projects", "list-11")

	view, _ := state.Viewport(9)
	plain := stripANSIString(view)
	if !strings.Contains(plain, "Metrics dashboard polish11") {
		t.Fatalf("expected bottom entry to be visible in viewport, got:\n%s", plain)
	}
}
func TestFormatEntryLinesIndentRendering(t *testing.T) {
	parent := &entry.Entry{ID: "p", Message: "Parent", Bullet: glyph.Task}
	child := &entry.Entry{ID: "c", Message: "Child", Bullet: glyph.Event, ParentID: "p"}
	grand := &entry.Entry{ID: "g", Message: "Grandchild", Bullet: glyph.Completed, ParentID: "c"}

	sections := []Section{{
		CollectionID: "Demo",
		Entries:      []*entry.Entry{parent, child, grand},
	}}

	state := NewState()
	state.SetWrapWidth(80)
	state.SetSections(sections)
	state.SetActive("Demo", "g")

	view, _ := state.Viewport(10)
	lines := strings.Split(view, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 rendered lines, got %d", len(lines))
	}

	var cleaned []string
	for _, line := range lines {
		plain := strings.TrimSpace(stripANSIString(line))
		if plain == "" {
			continue
		}
		cleaned = append(cleaned, plain)
	}
	if len(cleaned) < 3 {
		t.Fatalf("not enough rendered lines after stripping: %v", cleaned)
	}
	if !strings.Contains(cleaned[0], "⦁ Parent") {
		t.Fatalf("unexpected parent line: %q", cleaned[0])
	}
	if !strings.Contains(cleaned[1], "○ Child") {
		t.Fatalf("unexpected child line: %q", cleaned[1])
	}
	if !strings.Contains(cleaned[2], "✘ Grandchild") {
		t.Fatalf("unexpected grandchild line: %q", cleaned[2])
	}
}

func TestViewportPadsTrailingLines(t *testing.T) {
	entries := makeEntries(3)
	section := Section{
		CollectionID:   "A",
		CollectionName: "A",
		Entries:        entries,
	}

	state := NewState()
	state.SetSections([]Section{section})
	state.SetActive("A", entries[len(entries)-1].ID)

	height := 5
	view, _ := state.Viewport(height)
	lines := strings.Split(view, "\n")
	if len(lines) != height {
		t.Fatalf("expected viewport to yield %d lines, got %d", height, len(lines))
	}
	if lines[height-1] != "" {
		t.Fatalf("expected viewport to pad trailing lines with blanks, got %q", lines[height-1])
	}
}

func TestFormatEntryLinesAnnotatesLocked(t *testing.T) {
	locked := &entry.Entry{
		ID:        "locked",
		Message:   "Legacy task",
		Bullet:    glyph.MovedCollection,
		Immutable: true,
	}
	lines := formatEntryLines(locked, " ", "", 60)
	if len(lines) == 0 {
		t.Fatalf("expected at least one line for locked entry")
	}
	first := strings.TrimSpace(stripANSIString(lines[0]))
	if !strings.Contains(first, "Legacy task") {
		t.Fatalf("expected message in output, got %q", first)
	}
	if !strings.Contains(first, "locked") {
		t.Fatalf("expected locked annotation in output, got %q", first)
	}
}
