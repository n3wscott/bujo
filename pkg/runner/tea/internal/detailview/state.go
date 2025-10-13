package detailview

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/muesli/reflow/wordwrap"

	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
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
	if len(s.sections[s.sectionIndex].Entries) == 0 {
		return false
	}
	attempts := 0
	maxAttempts := len(s.sections) * 4
	for {
		section := &s.sections[s.sectionIndex]
		s.entryIndex += delta
		for s.entryIndex < 0 || s.entryIndex >= len(section.Entries) {
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
					break
				}
			} else {
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
		if s.isVisibleEntry(s.sectionIndex, s.entryIndex) {
			break
		}
		deltaSign := 1
		if delta < 0 {
			deltaSign = -1
		}
		s.entryIndex += deltaSign
		attempts++
		if attempts > maxAttempts {
			return false
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
	if len(s.sections[s.sectionIndex].Entries) == 0 {
		s.entryIndex = 0
	} else {
		if idx := s.firstVisibleIndex(s.sectionIndex); idx >= 0 {
			s.entryIndex = idx
		} else {
			s.entryIndex = s.clampedEntryIndex()
		}
	}
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
	s.ensureVisibleCurrent()
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
	if len(s.sections[s.sectionIndex].Entries) > 0 {
		entry := s.sections[s.sectionIndex].Entries[s.entryIndex]
		if entry != nil {
			s.unfoldAncestors(s.sectionIndex, entry.ID)
		}
	}
	s.ensureVisibleCurrent()
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

func (s *State) invalidateHeights() {
	for i := range s.cachedHeights {
		s.cachedHeights[i] = -1
	}
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
	if len(s.sections) == 0 || s.sectionIndex >= len(s.sections) {
		return
	}
	section := s.sections[s.sectionIndex]
	if len(section.Entries) == 0 {
		cursorRow += 1
	} else {
		row := s.visibleRow(s.sectionIndex, s.entryIndex)
		if row < 0 {
			row = 0
		}
		cursorRow += 1 + row
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

func (s *State) sectionTop(idx int) int {
	top := 0
	for i := 0; i < idx && i < len(s.sections); i++ {
		top += s.sectionHeight(i)
	}
	return top
}

func (s *State) renderSection(idx int) []string {
	if idx < 0 || idx >= len(s.sections) {
		return nil
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
			if !s.isVisibleEntry(idx, entryIdx) {
				continue
			}
			caret := " "
			if selected && entryIdx == s.entryIndex {
				caret = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Render("→")
			}
			indent := strings.Repeat("  ", s.depthOf(idx, item.ID))
			itemLines := formatEntryLines(item, caret, indent, s.wrapWidth)
			lines = append(lines, itemLines...)
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

func (s *State) isVisibleEntry(sectionIdx, entryIdx int) bool {
	if sectionIdx < 0 || sectionIdx >= len(s.sections) {
		return false
	}
	section := s.sections[sectionIdx]
	if entryIdx < 0 || entryIdx >= len(section.Entries) {
		return false
	}
	item := section.Entries[entryIdx]
	if item == nil {
		return false
	}
	visited := make(map[string]bool)
	parentID := item.ParentID
	for parentID != "" {
		if visited[parentID] {
			break
		}
		visited[parentID] = true
		if s.folded[parentID] {
			return false
		}
		next, ok := s.parents[sectionIdx][parentID]
		if !ok {
			break
		}
		parentID = next
	}
	return true
}

func (s *State) depthOf(sectionIdx int, entryID string) int {
	depth := 0
	visited := make(map[string]bool)
	current := entryID
	for {
		parentID, ok := s.parents[sectionIdx][current]
		if !ok || parentID == "" {
			break
		}
		if visited[parentID] {
			break
		}
		visited[parentID] = true
		depth++
		current = parentID
	}
	return depth
}

func (s *State) hasChildren(sectionIdx int, entryID string) bool {
	if sectionIdx < 0 || sectionIdx >= len(s.children) {
		return false
	}
	return len(s.children[sectionIdx][entryID]) > 0
}

func (s *State) EntryHasChildren(sectionIdx int, entryID string) bool {
	return s.hasChildren(sectionIdx, entryID)
}

func (s *State) ToggleEntryFold(entryID string, collapsed bool) {
	if s.folded == nil {
		s.folded = make(map[string]bool)
	}
	if collapsed {
		s.folded[entryID] = true
	} else {
		delete(s.folded, entryID)
	}
	s.invalidateHeights()
}

func (s *State) EntryFolded(entryID string) bool {
	return s.folded[entryID]
}

func (s *State) ensureVisibleCurrent() {
	if len(s.sections) == 0 {
		s.sectionIndex = 0
		s.entryIndex = 0
		return
	}
	if s.sectionIndex < 0 {
		s.sectionIndex = 0
	}
	if s.sectionIndex >= len(s.sections) {
		s.sectionIndex = len(s.sections) - 1
	}
	section := s.sections[s.sectionIndex]
	if len(section.Entries) == 0 {
		s.entryIndex = 0
		return
	}
	if s.entryIndex < 0 {
		s.entryIndex = 0
	}
	if s.entryIndex >= len(section.Entries) {
		s.entryIndex = len(section.Entries) - 1
	}
	if s.isVisibleEntry(s.sectionIndex, s.entryIndex) {
		return
	}
	if idx := s.firstVisibleIndex(s.sectionIndex); idx >= 0 {
		s.entryIndex = idx
	}
}

func (s *State) visibleRow(sectionIdx, entryIdx int) int {
	if !s.isVisibleEntry(sectionIdx, entryIdx) {
		return -1
	}
	count := 0
	for i := 0; i < entryIdx; i++ {
		if s.isVisibleEntry(sectionIdx, i) {
			count++
		}
	}
	return count
}

func (s *State) firstVisibleIndex(sectionIdx int) int {
	if sectionIdx < 0 || sectionIdx >= len(s.sections) {
		return -1
	}
	section := s.sections[sectionIdx]
	for i := range section.Entries {
		if s.isVisibleEntry(sectionIdx, i) {
			return i
		}
	}
	return -1
}

func (s *State) unfoldAncestors(sectionIdx int, entryID string) {
	visited := make(map[string]bool)
	current := entryID
	for {
		parentID, ok := s.parents[sectionIdx][current]
		if !ok || parentID == "" {
			break
		}
		if visited[parentID] {
			break
		}
		visited[parentID] = true
		delete(s.folded, parentID)
		current = parentID
	}
}

func (s *State) viewHeightFor(height int) int {
	if height > 0 {
		return height
	}
	if s.viewHeight > 0 {
		return s.viewHeight
	}
	return 25
}

// RevealCollection adjusts scrollOffset so the requested collection comes into
// view. When preferFull is true and the collection fits within the viewport we
// align the bottom edge so the entire section is visible; otherwise the header
// is pinned to the top once revealed.
func (s *State) RevealCollection(collectionID string, preferFull bool, height int) {
	idx := s.indexOfCollection(collectionID)
	if idx < 0 {
		return
	}
	viewport := s.viewHeightFor(height)
	top := s.sectionTop(idx)
	sectionHeight := s.sectionHeight(idx)
	if preferFull && sectionHeight <= viewport {
		target := top + sectionHeight - viewport
		if target < 0 {
			target = 0
		}
		s.scrollOffset = target
		return
	}
	s.scrollOffset = clampScrollOffset(top, s.maxScrollOffset(viewport))
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

func formatEntryLines(e *entry.Entry, caret, indent string, wrapWidth int) []string {
	signifier := e.Signifier.String()
	if signifier == "" {
		signifier = " "
	}
	bulletGlyph := e.Bullet.Glyph()
	bullet := bulletGlyph.Symbol
	if bullet == "" {
		bullet = e.Bullet.String()
	}
	message := e.Message
	if strings.TrimSpace(message) == "" {
		message = "<empty>"
	}
	msgLines := strings.Split(message, "\n")
	indentStr := indent
	bulletWithIndent := bullet
	if indentStr != "" {
		bulletWithIndent = indentStr + bullet
	}
	prefix := fmt.Sprintf("%s%s %s ", caret, signifier, bulletWithIndent)
	prefixStyle := lipgloss.NewStyle()
	messageStyle := lipgloss.NewStyle()
	if e.Bullet == glyph.Completed || e.Bullet == glyph.Irrelevant {
		prefixStyle = prefixStyle.Foreground(lipgloss.Color("241"))
		messageStyle = messageStyle.Foreground(lipgloss.Color("241"))
	}
	if e.Immutable {
		prefixStyle = prefixStyle.Foreground(lipgloss.Color("244")).Faint(true)
		messageStyle = messageStyle.Foreground(lipgloss.Color("244")).Faint(true).Italic(true)
	}
	if e.Bullet == glyph.Irrelevant {
		messageStyle = messageStyle.Strikethrough(true)
	}
	width := wrapWidth
	if width <= 0 {
		width = 80
	}
	available := width - lipgloss.Width(prefix)
	if available < 10 {
		available = 10
	}
	wrapLine := func(text string) []string {
		if strings.TrimSpace(text) == "" {
			return []string{text}
		}
		wrapped := wordwrap.String(text, available)
		if wrapped == "" {
			return []string{""}
		}
		return strings.Split(wrapped, "\n")
	}
	lines := make([]string, 0, len(msgLines))
	padding := strings.Repeat(" ", lipgloss.Width(prefix))
	paddingStyled := prefixStyle.Render(padding)
	firstLine := true
	lockedSuffix := ""
	if e.Immutable {
		lockedSuffix = " · locked"
	}
	for _, msgLine := range msgLines {
		segments := wrapLine(msgLine)
		for i, seg := range segments {
			content := seg
			if firstLine && i == 0 && lockedSuffix != "" {
				content = content + lockedSuffix
			}
			if firstLine && i == 0 {
				lines = append(lines, prefixStyle.Render(prefix)+messageStyle.Render(content))
				firstLine = false
				continue
			}
			lines = append(lines, paddingStyled+messageStyle.Render(content))
		}
	}
	if len(lines) == 0 {
		lines = append(lines, prefixStyle.Render(prefix))
	}
	return lines
}
