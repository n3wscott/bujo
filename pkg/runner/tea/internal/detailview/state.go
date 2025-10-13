package detailview

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/v2"

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
}

// NewState constructs an empty state.
func NewState() *State {
	return &State{}
}

// SetSections replaces the visible sections.
func (s *State) SetSections(sections []Section) {
	prevScroll := s.scrollOffset
	prevHeight := s.viewHeight

	s.sections = sections
	s.cachedHeights = make([]int, len(sections))
	for i := range s.cachedHeights {
		s.cachedHeights[i] = -1
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

func clampScrollOffset(offset, max int) int {
	if offset < 0 {
		offset = 0
	}
	if max < 0 {
		max = 0
	}
	if offset > max {
		return max
	}
	return offset
}

// Sections returns the currently loaded sections.
func (s *State) Sections() []Section {
	return s.sections
}

// Cursor returns the active section and entry indices.
func (s *State) Cursor() (int, int) {
	return s.sectionIndex, s.entryIndex
}

// MoveEntry moves the cursor within entries, adjusting section when crossing boundaries.
func (s *State) MoveEntry(delta int) bool {
	if len(s.sections) == 0 {
		return false
	}
	section := &s.sections[s.sectionIndex]
	s.entryIndex += delta
	for {
		if s.entryIndex >= 0 && s.entryIndex < len(section.Entries) {
			break
		}
		if s.entryIndex < 0 {
			if s.sectionIndex == 0 {
				s.entryIndex = 0
				break
			}
			s.sectionIndex--
			section = &s.sections[s.sectionIndex]
			s.entryIndex = len(section.Entries) - 1
			if s.entryIndex < 0 {
				s.entryIndex = 0
			}
		} else if s.entryIndex >= len(section.Entries) {
			if s.sectionIndex == len(s.sections)-1 {
				s.entryIndex = len(section.Entries) - 1
				if s.entryIndex < 0 {
					s.entryIndex = 0
				}
				break
			}
			s.sectionIndex++
			section = &s.sections[s.sectionIndex]
			s.entryIndex = 0
		}
	}
	s.ensureScrollVisible()
	return true
}

// MoveSection moves to another section and resets entry index.
func (s *State) MoveSection(delta int) bool {
	if len(s.sections) == 0 {
		return false
	}
	s.sectionIndex += delta
	if s.sectionIndex < 0 {
		s.sectionIndex = 0
	}
	if s.sectionIndex >= len(s.sections) {
		s.sectionIndex = len(s.sections) - 1
	}
	s.entryIndex = s.clampedEntryIndex()
	s.ensureScrollVisible()
	return true
}

// SetCursor positions the cursor, clamping to available entries.
func (s *State) SetCursor(sectionIdx, entryIdx int) {
	if len(s.sections) == 0 {
		s.sectionIndex = 0
		s.entryIndex = 0
		return
	}
	if sectionIdx < 0 {
		sectionIdx = 0
	}
	if sectionIdx >= len(s.sections) {
		sectionIdx = len(s.sections) - 1
	}
	s.sectionIndex = sectionIdx
	s.entryIndex = entryIdx
	s.clampEntry()
	s.ensureScrollVisible()
}

// SetActive moves the cursor to the given collection and entry identifiers.
func (s *State) SetActive(collectionID, entryID string) {
	if len(s.sections) == 0 {
		s.sectionIndex = 0
		s.entryIndex = 0
		return
	}
	if collectionID != "" {
		if idx := s.indexOfCollection(collectionID); idx >= 0 {
			s.sectionIndex = idx
		}
	}
	s.clampEntry()
	if entryID != "" {
		section := s.sections[s.sectionIndex]
		for i, it := range section.Entries {
			if it.ID == entryID {
				s.entryIndex = i
				break
			}
		}
	}
	s.clampEntry()
	s.ensureScrollVisible()
}

func (s *State) indexOfCollection(collectionID string) int {
	for i, sec := range s.sections {
		if sec.CollectionID == collectionID {
			return i
		}
	}
	return -1
}

func (s *State) clampEntry() {
	if len(s.sections) == 0 {
		s.entryIndex = 0
		return
	}
	s.entryIndex = s.clampedEntryIndex()
}

func (s *State) clampedEntryIndex() int {
	if len(s.sections) == 0 {
		return 0
	}
	entries := len(s.sections[s.sectionIndex].Entries)
	if entries == 0 {
		return 0
	}
	idx := s.entryIndex
	if idx < 0 {
		idx = 0
	}
	if idx >= entries {
		idx = entries - 1
	}
	return idx
}

// ensureScrollVisible adjusts scroll offset so active row is visible in viewport.
func (s *State) ensureScrollVisible() {
	height := s.viewHeight
	if height <= 0 {
		height = 25
	}
	contentTop := 0
	for i := 0; i < s.sectionIndex; i++ {
		contentTop += s.sectionHeight(i)
	}
	cursorRow := contentTop
	if len(s.sections[s.sectionIndex].Entries) == 0 {
		cursorRow += 1
	} else {
		cursorRow += 1 + s.entryIndex
	}
	if cursorRow < s.scrollOffset {
		s.scrollOffset = cursorRow
	}
	viewBottom := s.scrollOffset + height - 1
	if cursorRow > viewBottom {
		s.scrollOffset = cursorRow - height + 1
		if s.scrollOffset < 0 {
			s.scrollOffset = 0
		}
	}
}

// Viewport renders sections within height, returning string lines and content height.
func (s *State) Viewport(height int) (string, int) {
	if height <= 0 {
		return "", 0
	}
	s.viewHeight = height
	content := s.renderAll()
	if s.scrollOffset < 0 {
		s.scrollOffset = 0
	}
	if s.scrollOffset >= len(content) {
		s.scrollOffset = 0
	}
	end := s.scrollOffset + height
	if end > len(content) {
		end = len(content)
	}
	return strings.Join(content[s.scrollOffset:end], "\n"), len(content)
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

// ScrollToTop resets viewport to the first line.
func (s *State) ScrollToTop() {
	s.scrollOffset = 0
	s.sectionIndex = 0
	s.entryIndex = 0
}

func (s *State) renderAll() []string {
	var lines []string
	for idx := range s.sections {
		lines = append(lines, s.renderSection(idx)...)
	}
	return lines
}

func (s *State) renderSection(idx int) []string {
	if idx < 0 || idx >= len(s.sections) {
		return nil
	}
	if cached := s.cachedHeights[idx]; cached >= 0 {
		// rendering again is fine; cache is kept for viewport metrics.
	}
	section := s.sections[idx]
	header := formatCollectionTitle(section.CollectionName, section.ResolvedName)
	selected := idx == s.sectionIndex

	headerStyle := lipgloss.NewStyle().Bold(true)
	if selected {
		headerStyle = headerStyle.Foreground(lipgloss.Color("213"))
	}

	lines := []string{headerStyle.Render(header)}

	if len(section.Entries) == 0 {
		lines = append(lines, "  <empty>")
	} else {
		for entryIdx, item := range section.Entries {
			caret := " "
			if selected && entryIdx == s.entryIndex {
				caret = "→"
			}
			lines = append(lines, fmt.Sprintf("%s %s", caret, renderEntry(item)))
		}
	}
	lines = append(lines, "") // spacer between sections
	s.cachedHeights[idx] = len(lines)
	return lines
}

func (s *State) sectionHeight(idx int) int {
	if idx < 0 || idx >= len(s.sections) {
		return 0
	}
	if s.cachedHeights[idx] >= 0 {
		return s.cachedHeights[idx]
	}
	lines := s.renderSection(idx)
	return len(lines)
}

func (s *State) maxScrollOffset(height int) int {
	if height <= 0 {
		return 0
	}
	total := 0
	for i := range s.sections {
		total += s.sectionHeight(i)
	}
	max := total - height
	if max < 0 {
		return 0
	}
	return max
}

func formatCollectionTitle(name, resolved string) string {
	if resolved != "" {
		if strings.Contains(resolved, "/") {
			parts := strings.SplitN(resolved, "/", 2)
			if len(parts) == 2 {
				if t, err := time.Parse("January 2, 2006", parts[1]); err == nil {
					return t.Format("Monday, January 2, 2006")
				}
				if mt, err := time.Parse("January 2006", parts[0]); err == nil {
					return mt.Format("January, 2006")
				}
			}
		}
		if t, err := time.Parse("January 2, 2006", resolved); err == nil {
			return t.Format("Monday, January 2, 2006")
		}
		if t, err := time.Parse("January 2006", resolved); err == nil {
			return t.Format("January, 2006")
		}
	}
	if t, err := time.Parse("January 2, 2006", name); err == nil {
		return t.Format("Monday, January 2, 2006")
	}
	if t, err := time.Parse("January 2006", name); err == nil {
		return t.Format("January, 2006")
	}
	return name
}

func renderEntry(e *entry.Entry) string {
	signifier := e.Signifier.String()
	bullet := e.Bullet.String()
	message := e.Message
	if signifier == "" {
		signifier = " "
	}
	return fmt.Sprintf("%s %s  %s", signifier, bullet, message)
}
