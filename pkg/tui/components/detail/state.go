// Package detail manages the per-panel entry state for the TUI detail pane.
package detail

import (
	"tableflip.dev/bujo/pkg/entry"
)

// Section represents a collection and its entries rendered in the detail pane.
type Section struct {
	CollectionID   string
	CollectionName string
	ResolvedName   string
	Entries        []*entry.Entry
}

// State tracks visible sections and cursor position.
type State struct {
	sections []Section
	// position inside sections
	sectionIndex int
	entryIndex   int
	// virtual scroll offset in rows within rendered content
	scrollOffset int
	// cached heights per section to avoid recomputing sizes on every frame
	cachedHeights []int
	viewHeight    int
	wrapWidth     int
	folded        map[string]bool
	parents       []map[string]string
	children      []map[string][]*entry.Entry
	entryOffsets  [][]int
	entryHeights  [][]int
}

// NewState constructs an empty state.
func NewState() *State {
	return &State{folded: make(map[string]bool), wrapWidth: 80}
}

// SetSections replaces the visible sections.
func (s *State) SetSections(sections []Section) {
	prevScroll := s.scrollOffset
	prevHeight := s.viewHeight

	s.sections = sections
	s.cachedHeights = make([]int, len(sections))
	s.entryOffsets = make([][]int, len(sections))
	s.entryHeights = make([][]int, len(sections))
	s.parents = make([]map[string]string, len(sections))
	s.children = make([]map[string][]*entry.Entry, len(sections))
	for i := range s.cachedHeights {
		s.cachedHeights[i] = -1
	}
	for i := range sections {
		s.parents[i], s.children[i] = buildRelations(sections[i].Entries)
	}

	if len(sections) == 0 {
		s.sectionIndex = 0
		s.entryIndex = 0
		s.scrollOffset = 0
		return
	}

	if s.sectionIndex >= len(sections) {
		s.sectionIndex = len(sections) - 1
	}
	if s.sectionIndex < 0 {
		s.sectionIndex = 0
	}
	s.clampEntry()

	s.scrollOffset = clampScrollOffset(prevScroll, s.maxScrollOffset(prevHeight))
}

// SetWrapWidth controls the wrapping width for rendered entries.
func (s *State) SetWrapWidth(width int) {
	if width <= 0 {
		width = 80
	}
	if width == s.wrapWidth {
		return
	}
	s.wrapWidth = width
	s.invalidateHeights()
}

// Sections returns the currently loaded sections.
func (s *State) Sections() []Section {
	return s.sections
}

// Cursor returns the active section and entry indices.
func (s *State) Cursor() (int, int) {
	return s.sectionIndex, s.entryIndex
}

// ActiveEntryID returns the entry ID currently highlighted.
func (s *State) ActiveEntryID() string {
	if len(s.sections) == 0 {
		return ""
	}
	section := s.sections[s.sectionIndex]
	if len(section.Entries) == 0 {
		return ""
	}
	return section.Entries[s.entryIndex].ID
}

// ActiveCollectionID returns the collection for the cursor.
func (s *State) ActiveCollectionID() string {
	if len(s.sections) == 0 {
		return ""
	}
	return s.sections[s.sectionIndex].CollectionID
}

func buildRelations(entries []*entry.Entry) (map[string]string, map[string][]*entry.Entry) {
	parents := make(map[string]string, len(entries))
	children := make(map[string][]*entry.Entry)
	idSet := make(map[string]*entry.Entry, len(entries))
	for _, e := range entries {
		if e == nil || e.ID == "" {
			continue
		}
		idSet[e.ID] = e
	}
	for _, e := range entries {
		if e == nil || e.ID == "" {
			continue
		}
		parents[e.ID] = e.ParentID
		if e.ParentID == "" {
			continue
		}
		if _, ok := idSet[e.ParentID]; !ok {
			continue
		}
		children[e.ParentID] = append(children[e.ParentID], e)
	}
	return parents, children
}
