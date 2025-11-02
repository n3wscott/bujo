package collectiondetail

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/muesli/reflow/wordwrap"

	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/tui/events"
)

// Bullet describes a single entry row inside a collection detail section.
type Bullet struct {
	ID        string
	Label     string
	Note      string
	Bullet    glyph.Bullet
	Signifier glyph.Signifier
	Created   time.Time
	Children  []Bullet
}

// Section groups a set of bullets under a collection title.
type Section struct {
	ID          string
	Title       string
	Subtitle    string
	Bullets     []Bullet
	Placeholder bool
}

// Model renders a scrollable list of section headers with their bullets.
type Model struct {
	sections []Section
	lookup   map[string]int

	width    int
	height   int
	debugLog io.Writer

	cursor           int // index into bulletLines, -1 when nothing selectable
	scroll           int
	activeSection    int
	pendingSectionID string
	pendingBulletID  string

	focused bool

	lines         []lineInfo
	bulletLines   []int
	lineHeights   []int
	lineOffsets   []int
	totalHeight   int
	id            events.ComponentID
	lastHighlight string
	sourceNav     events.ComponentID
}

const (
	lineHeader = -1
	lineSpacer = -2
	lineEmpty  = -3
	lineItem   = -4
)

type lineInfo struct {
	section int
	kind    int // >=0 bullet index, otherwise line constants above
	indent  int
	bullet  Bullet
}

// NewModel constructs the detail component with the provided sections.
func NewModel(sections []Section) *Model {
	m := &Model{cursor: -1, activeSection: -1, id: events.ComponentID("collectiondetail")}
	m.SetSections(sections)
	return m
}

// SetDebugWriter configures an optional writer for diagnostic output.
func (m *Model) SetDebugWriter(w io.Writer) {
	m.debugLog = w
}

// SetSections replaces the rendered sections.
func (m *Model) SetSections(sections []Section) {
	m.sections = append([]Section(nil), sections...)
	for i := range m.sections {
		if len(m.sections[i].Bullets) > 0 {
			m.sections[i].Placeholder = false
		}
	}
	m.rebuildLookup()
	m.refreshFromSections(true)
}

func (m *Model) rebuildLookup() {
	if m.lookup == nil {
		m.lookup = make(map[string]int)
	} else {
		for k := range m.lookup {
			delete(m.lookup, k)
		}
	}
	for idx, sec := range m.sections {
		if sec.ID != "" {
			m.lookup["id:"+strings.ToLower(sec.ID)] = idx
		}
		if sec.Title != "" {
			m.lookup["title:"+strings.ToLower(sec.Title)] = idx
		}
	}
}

func (m *Model) refreshFromSections(resetHighlight bool) {
	m.rebuildLines()
	if len(m.bulletLines) == 0 {
		m.cursor = -1
	} else if m.cursor < 0 || m.cursor >= len(m.bulletLines) {
		m.cursor = 0
	}
	m.ensureScroll()
	m.refreshActiveSection()
	m.applyPendingSelection()
	if resetHighlight {
		m.lastHighlight = ""
	}
}

// SetSourceNav configures which nav component drives highlight events for this
// detail pane. When empty, all highlight events are accepted.
func (m *Model) SetSourceNav(id events.ComponentID) {
	m.sourceNav = id
}

// SetSize configures the viewport dimensions.
func (m *Model) SetSize(width, height int) {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 20
	}
	m.width = width
	m.height = height
	m.recomputeLineMetrics()
	if m.debugLog != nil {
		fmt.Fprintf(m.debugLog, "%s detail.SetSize width=%d height=%d\n",
			time.Now().Format("2006-01-02T15:04:05"), width, height)
	}
	m.ensureScroll()
}

// Focus marks the component as active (highlights the cursor line).
func (m *Model) Focus() tea.Cmd {
	if m.focused {
		return nil
	}
	m.focused = true
	return events.FocusCmd(m.id)
}

// Blur marks the component as inactive.
func (m *Model) Blur() tea.Cmd {
	if !m.focused {
		return nil
	}
	m.focused = false
	return events.BlurCmd(m.id)
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// Update handles key presses for navigation.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}
		switch msg.String() {
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "pgup", "b":
			m.moveCursor(-m.pageSize())
		case "pgdown", "f":
			m.moveCursor(m.pageSize())
		case "home", "g":
			if len(m.bulletLines) > 0 {
				m.cursor = 0
				m.ensureScroll()
				m.refreshActiveSection()
			}
		case "end", "G":
			if len(m.bulletLines) > 0 {
				m.cursor = len(m.bulletLines) - 1
				m.ensureScroll()
				m.refreshActiveSection()
			}
		case "enter", " ":
			if cmd := m.selectCmd(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	case events.CollectionHighlightMsg:
		if m.sourceNav == "" || m.sourceNav == msg.Component {
			m.focusSectionForCollection(msg.Collection)
		}
	case events.CollectionChangeMsg:
		if m.applyCollectionChange(msg) {
			m.rebuildLookup()
			m.refreshFromSections(false)
		}
	case events.BulletChangeMsg:
		if m.applyBulletChange(msg) {
			m.refreshFromSections(false)
		}
	case events.CollectionSelectMsg:
		if m.sourceNav == "" || m.sourceNav == msg.Component {
			if !msg.Exists {
				m.ensurePlaceholderSection(msg.Collection)
				m.focusSectionForCollection(msg.Collection)
			}
		}
	case events.CollectionOrderMsg:
		if m.reorderSections(msg.Order) {
			m.rebuildLookup()
			m.refreshFromSections(false)
		}
	}

	if cmd := m.highlightCmd(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

// View renders the component.
func (m *Model) View() string {
	if m.height <= 0 {
		m.height = 20
	}
	if m.width <= 0 {
		m.width = 80
	}

	lines := m.renderVisibleLines()
	if m.debugLog != nil {
		cursorLine := -1
		if m.cursor >= 0 && m.cursor < len(m.bulletLines) {
			cursorLine = m.bulletLines[m.cursor]
		}
		fmt.Fprintf(m.debugLog, "%s detail.View lines=%d cursorLine=%d scroll=%d height=%d\n",
			time.Now().Format("2006-01-02T15:04:05"), len(lines), cursorLine, m.scroll, m.height)
	}
	return strings.Join(lines, "\n")
}

func (m *Model) visibleSection() (int, bool) {
	if len(m.lines) == 0 || len(m.sections) == 0 {
		return -1, false
	}
	start := m.scroll
	if start < 0 {
		start = 0
	}
	if start >= len(m.lines) {
		start = len(m.lines) - 1
	}
	for i := start; i < len(m.lines); i++ {
		info := m.lines[i]
		if info.section < 0 || info.section >= len(m.sections) {
			continue
		}
		if info.kind == lineSpacer {
			continue
		}
		return info.section, true
	}
	return -1, false
}

func (m *Model) moveCursor(delta int) {
	if len(m.bulletLines) == 0 {
		m.cursor = -1
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.bulletLines) {
		m.cursor = len(m.bulletLines) - 1
	}
	m.ensureScroll()
	m.refreshActiveSection()
}

func (m *Model) ensureScroll() {
	if len(m.lines) == 0 {
		m.scroll = 0
		return
	}
	curLine := m.currentLineIndex()
	if curLine < 0 {
		m.scroll = 0
		m.clampScroll()
		return
	}
	m.ensureLineVisible(curLine)
}

func (m *Model) pageSize() int {
	height := m.viewportContentHeight()
	if height <= 0 {
		return 10
	}
	if height <= 1 {
		return 1
	}
	return height - 1
}

func (m *Model) ensureLineVisible(target int) {
	if len(m.lines) == 0 {
		m.scroll = 0
		return
	}
	if target < 0 {
		target = 0
	}
	if target >= len(m.lines) {
		target = len(m.lines) - 1
	}
	contentHeight := m.viewportContentHeight()
	if contentHeight <= 0 {
		contentHeight = 1
	}
	topIdx := m.scroll
	if topIdx < 0 {
		topIdx = 0
	}
	if topIdx >= len(m.lines) {
		topIdx = len(m.lines) - 1
	}
	topOffset := m.lineOffset(topIdx)
	bottomOffset := topOffset
	remaining := contentHeight
	idx := topIdx
	for idx < len(m.lines) && remaining > 0 {
		h := m.lineHeight(idx)
		if h >= remaining {
			bottomOffset = m.lineOffset(idx) + remaining - 1
			remaining = 0
			break
		}
		remaining -= h
		bottomOffset = m.lineOffset(idx) + h - 1
		idx++
	}
	if remaining > 0 {
		bottomOffset = m.totalHeight - 1
	}
	lineTop := m.lineOffset(target)
	lineBottom := lineTop + m.lineHeight(target) - 1
	if lineTop < topOffset {
		m.scroll = target
		m.clampScroll()
		return
	}
	if lineBottom > bottomOffset {
		start := target
		total := m.lineHeight(target)
		if total <= 0 {
			total = 1
		}
		for start > 0 {
			prev := start - 1
			nextTotal := total + m.lineHeight(prev)
			if nextTotal > contentHeight {
				break
			}
			start = prev
			total = nextTotal
		}
		m.scroll = start
		m.clampScroll()
		return
	}
	m.clampScroll()
}

func (m *Model) viewportContentHeight() int {
	if m.height <= 0 {
		return 0
	}
	height := m.height - m.stickyHeaderHeight()
	if height <= 0 {
		return 1
	}
	return height
}

func (m *Model) stickyHeaderHeight() int {
	if m.height <= 0 {
		return 0
	}
	section, ok := m.visibleSection()
	if !ok {
		return 0
	}
	header := m.renderSectionHeader(section, m.sectionActive(section))
	lines := strings.Count(header, "\n") + 1
	if lines < 0 {
		return 0
	}
	if lines >= m.height {
		return m.height - 1
	}
	return lines
}

func (m *Model) rebuildLines() {
	m.lines = m.lines[:0]
	m.bulletLines = m.bulletLines[:0]
	for si, sec := range m.sections {
		m.lines = append(m.lines, lineInfo{section: si, kind: lineHeader})
		if len(sec.Bullets) == 0 {
			lineIdx := len(m.lines)
			m.lines = append(m.lines, lineInfo{section: si, kind: lineEmpty})
			m.bulletLines = append(m.bulletLines, lineIdx)
		} else {
			m.appendBulletLines(si, sec.Bullets, 0)
		}
		m.lines = append(m.lines, lineInfo{section: si, kind: lineSpacer})
	}
	if len(m.lines) > 0 {
		m.lines = m.lines[:len(m.lines)-1]
	}
	m.recomputeLineMetrics()
}

func (m *Model) appendBulletLines(section int, bullets []Bullet, depth int) {
	for bi := range bullets {
		lineIdx := len(m.lines)
		bullet := bullets[bi]
		info := lineInfo{section: section, kind: lineItem, indent: depth, bullet: bullet}
		m.lines = append(m.lines, info)
		m.bulletLines = append(m.bulletLines, lineIdx)
		if len(bullet.Children) > 0 {
			m.appendBulletLines(section, bullet.Children, depth+1)
		}
	}
}

func (m *Model) recomputeLineMetrics() {
	n := len(m.lines)
	if n == 0 {
		m.lineHeights = m.lineHeights[:0]
		m.lineOffsets = m.lineOffsets[:0]
		m.totalHeight = 0
		m.scroll = 0
		return
	}
	if cap(m.lineHeights) < n {
		m.lineHeights = make([]int, n)
	} else {
		m.lineHeights = m.lineHeights[:n]
	}
	if cap(m.lineOffsets) < n {
		m.lineOffsets = make([]int, n)
	} else {
		m.lineOffsets = m.lineOffsets[:n]
	}
	offset := 0
	for i := 0; i < n; i++ {
		h := m.measureLineHeight(m.lines[i])
		if h <= 0 {
			h = 1
		}
		m.lineHeights[i] = h
		m.lineOffsets[i] = offset
		offset += h
	}
	m.totalHeight = offset
	m.clampScroll()
}

func (m *Model) measureLineHeight(info lineInfo) int {
	switch info.kind {
	case lineHeader:
		header := m.renderSectionHeader(info.section, false)
		return strings.Count(header, "\n") + 1
	case lineSpacer:
		return 1
	case lineEmpty:
		text := m.renderEmptyLine(info.section, false)
		return strings.Count(text, "\n") + 1
	case lineItem:
		prefix := m.composeBulletPrefix(info.indent, info.bullet, false)
		lines := m.renderBulletLines(prefix, info.bullet)
		if len(lines) == 0 {
			return 1
		}
		return len(lines)
	default:
		return 1
	}
}

func (m *Model) lineHeight(idx int) int {
	if idx < 0 || idx >= len(m.lineHeights) {
		return 0
	}
	h := m.lineHeights[idx]
	if h <= 0 {
		h = m.measureLineHeight(m.lines[idx])
		if h <= 0 {
			h = 1
		}
		m.lineHeights[idx] = h
	}
	return h
}

func (m *Model) lineOffset(idx int) int {
	if idx < 0 || idx >= len(m.lineOffsets) {
		return 0
	}
	return m.lineOffsets[idx]
}

func (m *Model) clampScroll() {
	if len(m.lines) == 0 {
		m.scroll = 0
		return
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
	if m.scroll >= len(m.lines) {
		m.scroll = len(m.lines) - 1
	}
	maxIdx := m.maxScrollIndex()
	if m.scroll > maxIdx {
		m.scroll = maxIdx
	}
}

func (m *Model) maxScrollIndex() int {
	if len(m.lines) == 0 {
		return 0
	}
	visible := m.viewportContentHeight()
	if visible <= 0 {
		return 0
	}
	if m.totalHeight <= visible {
		return 0
	}
	maxOffset := m.totalHeight - visible
	idx := sort.Search(len(m.lineOffsets), func(i int) bool {
		return m.lineOffsets[i] > maxOffset
	}) - 1
	if idx < 0 {
		idx = 0
	}
	return idx
}

// SetID overrides the emitted component identifier.
func (m *Model) SetID(id events.ComponentID) {
	if id == "" {
		m.id = events.ComponentID("collectiondetail")
		return
	}
	m.id = id
}

// ID returns the component identifier.
func (m *Model) ID() events.ComponentID {
	return m.id
}

func (m *Model) renderLine(idx int, selected bool) string {
	if idx < 0 || idx >= len(m.lines) {
		return ""
	}
	info := m.lines[idx]
	if info.section < 0 || info.section >= len(m.sections) {
		return ""
	}
	switch info.kind {
	case lineHeader:
		return m.renderSectionHeader(info.section, m.sectionActive(info.section))
	case lineSpacer:
		return ""
	case lineEmpty:
		return m.renderEmptyLine(info.section, m.sectionActive(info.section))
	case lineItem:
		return m.renderBulletInfo(info, selected)
	default:
		return ""
	}
}

func (m *Model) renderSectionHeader(section int, highlight bool) string {
	sec := m.sections[section]
	style := lipgloss.NewStyle().Bold(true)
	if sec.Placeholder {
		style = style.Italic(true).Foreground(lipgloss.Color("244"))
	}
	if highlight {
		style = style.Foreground(lipgloss.Color("213"))
	}
	title := sec.Title
	if title == "" {
		title = "(untitled)"
	}
	if sec.Subtitle != "" {
		title = title + " ▸ " + sec.Subtitle
	}
	return style.Width(m.width).Render(title)
}

func (m *Model) renderEmptyLine(section int, highlight bool) string {
	if section < 0 || section >= len(m.sections) {
		return ""
	}
	sec := m.sections[section]
	message := "  <empty>"
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	if sec.Placeholder {
		message = "  (collection not yet created — add a bullet to save it)"
		style = style.Italic(true).Foreground(lipgloss.Color("244"))
	}
	if highlight {
		style = style.Foreground(lipgloss.Color("213"))
	}
	return style.Render(message)
}

func (m *Model) renderBulletInfo(info lineInfo, selected bool) string {
	item := info.bullet
	prefix := m.composeBulletPrefix(info.indent, item, selected && m.focused)
	lines := m.renderBulletLines(prefix, item)
	prefixStyle, messageStyle := m.bulletStyles(item)
	for i, line := range lines {
		if i == 0 {
			lines[i] = prefixStyle.Render(prefix) + messageStyle.Render(strings.TrimPrefix(line, prefix))
		} else {
			lines[i] = messageStyle.Render(line)
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) wrapBulletLines(prefix, text string) []string {
	prefixWidth := lipgloss.Width(prefix)
	if prefixWidth <= 0 {
		prefixWidth = 2
	}
	available := m.width - prefixWidth
	if available < 10 {
		available = 10
	}

	wrapLine := func(s string) []string {
		if strings.TrimSpace(s) == "" {
			return []string{""}
		}
		wrapped := wordwrap.String(s, available)
		if wrapped == "" {
			return []string{""}
		}
		return strings.Split(wrapped, "\n")
	}

	padding := strings.Repeat(" ", prefixWidth)
	lines := make([]string, 0, 4)
	firstLine := true
	for _, raw := range strings.Split(text, "\n") {
		segments := wrapLine(raw)
		for i, seg := range segments {
			if firstLine && i == 0 {
				lines = append(lines, prefix+seg)
				continue
			}
			lines = append(lines, padding+seg)
		}
		firstLine = false
	}
	if len(lines) == 0 {
		lines = append(lines, prefix)
	}
	return lines
}

func (m *Model) renderBulletLabel(item Bullet) string {
	label := item.Label
	if strings.TrimSpace(label) == "" {
		label = "<empty>"
	}
	return label
}

func (m *Model) renderBulletLines(prefix string, item Bullet) []string {
	text := m.renderBulletLabel(item)
	return m.wrapBulletLines(prefix, text)
}

func (m *Model) bulletStyles(item Bullet) (lipgloss.Style, lipgloss.Style) {
	prefixStyle := lipgloss.NewStyle()
	messageStyle := lipgloss.NewStyle()
	switch item.Bullet {
	case glyph.Completed, glyph.Irrelevant, glyph.MovedCollection, glyph.MovedFuture:
		prefixStyle = prefixStyle.Foreground(lipgloss.Color("241"))
		messageStyle = messageStyle.Foreground(lipgloss.Color("241"))
	}
	if item.Bullet == glyph.Irrelevant {
		messageStyle = messageStyle.Strikethrough(true)
	}
	return prefixStyle, messageStyle
}

func (m *Model) composeBulletPrefix(depth int, item Bullet, selected bool) string {
	caret := " "
	if selected {
		caret = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Render("→")
	}
	signifier := item.Signifier.String()
	if signifier == "" {
		signifier = " "
	}
	indent := strings.Repeat("  ", depth)
	symbol := item.Bullet.Glyph().Symbol
	if symbol == "" {
		symbol = item.Bullet.String()
	}
	if symbol == "" {
		symbol = "-"
	}
	return caret + signifier + " " + indent + symbol + " "
}

func (m *Model) renderVisibleLines() []string {
	height := m.height
	if height <= 0 {
		height = 1
	}
	lines := make([]string, 0, height)

	stickySection, hasSticky := m.visibleSection()
	stickyHeight := 0

	appendLines := func(text string) {
		if len(lines) >= height {
			return
		}
		if text == "" {
			lines = append(lines, "")
			return
		}
		for _, part := range strings.Split(text, "\n") {
			if len(lines) >= height {
				break
			}
			lines = append(lines, part)
		}
	}

	skippedHeader := hasSticky
	if hasSticky {
		header := m.renderSectionHeader(stickySection, m.sectionActive(stickySection))
		stickyHeight = strings.Count(header, "\n") + 1
		appendLines(header)
	}

	start := m.scroll
	activeLine := m.currentLineIndex()
	for i := start; i < len(m.lines) && len(lines) < height; i++ {
		info := m.lines[i]
		if hasSticky && skippedHeader && info.kind == lineHeader && info.section == stickySection {
			skippedHeader = false
			continue
		}
		appendLines(m.renderLine(i, i == activeLine))
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	if m.debugLog != nil {
		cursorLine := -1
		if m.cursor >= 0 && m.cursor < len(m.bulletLines) {
			cursorLine = m.bulletLines[m.cursor]
		}
		fmt.Fprintf(m.debugLog, "%s detail.view lines=%d stickyHeight=%d cursorLine=%d scroll=%d height=%d\n",
			time.Now().Format("2006-01-02T15:04:05"), len(lines), stickyHeight, cursorLine, m.scroll, m.height)
	}

	return lines
}

func (m *Model) currentLineIndex() int {
	if m.cursor < 0 || m.cursor >= len(m.bulletLines) {
		return -1
	}
	return m.bulletLines[m.cursor]
}

func (m *Model) sectionActive(section int) bool {
	return section >= 0 && section < len(m.sections) && section == m.activeSection
}

func (m *Model) sectionIndexForCollection(ref events.CollectionRef) int {
	if idx := m.lookupIndex("id", ref.ID); idx >= 0 {
		return idx
	}
	if idx := m.lookupIndex("title", ref.Name); idx >= 0 {
		return idx
	}
	return -1
}

func (m *Model) sectionIndexForView(ref events.CollectionViewRef) int {
	if idx := m.lookupIndex("id", ref.ID); idx >= 0 {
		return idx
	}
	if idx := m.lookupIndex("title", ref.Title); idx >= 0 {
		return idx
	}
	return -1
}

func (m *Model) lookupIndex(kind, value string) int {
	if value == "" || m.lookup == nil {
		return -1
	}
	if idx, ok := m.lookup[kind+":"+strings.ToLower(value)]; ok {
		return idx
	}
	return -1
}

func (m *Model) focusSectionForCollection(ref events.CollectionRef) bool {
	sectionIdx := m.sectionIndexForCollection(ref)
	if sectionIdx < 0 {
		return false
	}
	targetLine := -1
	firstBulletLine := -1
	for idx, info := range m.lines {
		if info.section != sectionIdx {
			continue
		}
		if targetLine == -1 {
			targetLine = idx
		}
		if info.kind == lineItem {
			firstBulletLine = idx
			break
		}
	}
	if targetLine == -1 {
		return false
	}
	if firstBulletLine >= 0 {
		for cursorIdx, line := range m.bulletLines {
			if line == firstBulletLine {
				m.cursor = cursorIdx
				break
			}
		}
		m.ensureScroll()
		m.refreshActiveSection()
		return true
	}
	m.cursor = -1
	m.activeSection = sectionIdx
	m.scrollToLine(targetLine)
	return true
}

func (m *Model) scrollToLine(line int) {
	if line < 0 || line >= len(m.lines) {
		return
	}
	m.ensureLineVisible(line)
}

func (m *Model) currentBulletInfo() (lineInfo, Section, bool) {
	lineIdx := m.currentLineIndex()
	if lineIdx < 0 || lineIdx >= len(m.lines) {
		return lineInfo{}, Section{}, false
	}
	info := m.lines[lineIdx]
	if info.kind != lineItem || info.section < 0 || info.section >= len(m.sections) {
		return lineInfo{}, Section{}, false
	}
	return info, m.sections[info.section], true
}

// CurrentSelection returns the active section and, when available, the
// highlighted bullet. The boolean result reports whether a section is active.
func (m *Model) CurrentSelection() (Section, Bullet, bool) {
	if len(m.sections) == 0 {
		return Section{}, Bullet{}, false
	}
	if info, section, ok := m.currentBulletInfo(); ok {
		return section, info.bullet, true
	}
	secIdx := m.activeSection
	if secIdx < 0 || secIdx >= len(m.sections) {
		secIdx = 0
	}
	return m.sections[secIdx], Bullet{}, true
}

func (m *Model) highlightCmd() tea.Cmd {
	if !m.focused {
		return nil
	}
	info, section, ok := m.currentBulletInfo()
	if !ok {
		return m.highlightEmptySectionCmd()
	}
	key := highlightKey(info)
	if key == m.lastHighlight {
		return nil
	}
	m.lastHighlight = key
	return bulletHighlightCmd(m.id, section, info.bullet)
}

func (m *Model) highlightEmptySectionCmd() tea.Cmd {
	if !m.focused {
		return nil
	}
	if m.activeSection < 0 || m.activeSection >= len(m.sections) {
		if m.lastHighlight != "" {
			m.lastHighlight = ""
		}
		return nil
	}
	key := fmt.Sprintf("section:%d", m.activeSection)
	if key == m.lastHighlight {
		return nil
	}
	m.lastHighlight = key
	return bulletHighlightCmd(m.id, m.sections[m.activeSection], Bullet{})
}

func (m *Model) refreshActiveSection() {
	switch {
	case len(m.sections) == 0:
		m.activeSection = -1
		return
	case len(m.bulletLines) == 0:
		if m.activeSection < 0 || m.activeSection >= len(m.sections) {
			m.activeSection = 0
		}
		return
	}
	if m.cursor < 0 || m.cursor >= len(m.bulletLines) {
		m.cursor = 0
	}
	lineIdx := m.bulletLines[m.cursor]
	if lineIdx < 0 || lineIdx >= len(m.lines) {
		m.activeSection = 0
		return
	}
	sec := m.lines[lineIdx].section
	if sec < 0 || sec >= len(m.sections) {
		m.activeSection = 0
		return
	}
	m.activeSection = sec
}

func (m *Model) selectCmd() tea.Cmd {
	info, section, ok := m.currentBulletInfo()
	if !ok {
		return nil
	}
	return bulletSelectCmd(m.id, section, info.bullet)
}

func (m *Model) applyPendingSelection() {
	if m.pendingSectionID == "" {
		return
	}
	sectionID := m.pendingSectionID
	bulletID := m.pendingBulletID
	if bulletID != "" {
		if m.focusBulletByID(sectionID, bulletID) {
			m.pendingSectionID = ""
			m.pendingBulletID = ""
		}
		return
	}
	if m.focusBulletByID(sectionID, "") {
		m.pendingSectionID = ""
		return
	}
	if m.focusSectionByID(sectionID) {
		m.pendingSectionID = ""
	}
}

func (m *Model) focusBulletByID(sectionID, bulletID string) bool {
	sectionID = strings.TrimSpace(sectionID)
	if sectionID == "" {
		return false
	}
	secIdx := m.lookupIndex("id", sectionID)
	if secIdx < 0 || secIdx >= len(m.sections) {
		return false
	}
	trimmedBullet := strings.TrimSpace(bulletID)
	targetLine := -1
	for idx, info := range m.lines {
		if info.section != secIdx {
			continue
		}
		if trimmedBullet != "" {
			if info.kind == lineItem && strings.EqualFold(info.bullet.ID, trimmedBullet) {
				targetLine = idx
				break
			}
		} else if info.kind == lineItem {
			targetLine = idx
			break
		}
	}
	if targetLine == -1 {
		return false
	}
	for cursorIdx, lineIdx := range m.bulletLines {
		if lineIdx == targetLine {
			m.cursor = cursorIdx
			m.ensureScroll()
			m.refreshActiveSection()
			return true
		}
	}
	return false
}

func (m *Model) focusSectionByID(sectionID string) bool {
	sectionID = strings.TrimSpace(sectionID)
	if sectionID == "" {
		return false
	}
	secIdx := m.lookupIndex("id", sectionID)
	if secIdx < 0 || secIdx >= len(m.sections) {
		return false
	}
	for idx, info := range m.lines {
		if info.section == secIdx && info.kind == lineHeader {
			m.scrollToLine(idx)
			break
		}
	}
	m.activeSection = secIdx
	firstCursor := -1
	for cursorIdx, lineIdx := range m.bulletLines {
		info := m.lines[lineIdx]
		if info.section == secIdx {
			firstCursor = cursorIdx
			break
		}
	}
	m.cursor = firstCursor
	if m.cursor >= 0 {
		m.ensureScroll()
	}
	return true
}

func highlightKey(info lineInfo) string {
	bulletID := strings.TrimSpace(info.bullet.ID)
	if bulletID != "" {
		return bulletID
	}
	return fmt.Sprintf("%d:%s:%s", info.section, info.bullet.Label, info.bullet.Note)
}

func bulletHighlightCmd(component events.ComponentID, section Section, bullet Bullet) tea.Cmd {
	sectionRef := events.CollectionViewRef{
		ID:       section.ID,
		Title:    section.Title,
		Subtitle: section.Subtitle,
	}
	bulletRef := events.BulletRef{
		ID:        bullet.ID,
		Label:     bullet.Label,
		Note:      bullet.Note,
		Bullet:    bullet.Bullet,
		Signifier: bullet.Signifier,
	}
	return func() tea.Msg {
		return events.BulletHighlightMsg{
			Component:  component,
			Collection: sectionRef,
			Bullet:     bulletRef,
		}
	}
}

func bulletSelectCmd(component events.ComponentID, section Section, bullet Bullet) tea.Cmd {
	sectionRef := events.CollectionViewRef{
		ID:       section.ID,
		Title:    section.Title,
		Subtitle: section.Subtitle,
	}
	bulletRef := events.BulletRef{
		ID:        bullet.ID,
		Label:     bullet.Label,
		Note:      bullet.Note,
		Bullet:    bullet.Bullet,
		Signifier: bullet.Signifier,
	}
	exists := bullet.ID != ""
	return func() tea.Msg {
		return events.BulletSelectMsg{
			Component:  component,
			Collection: sectionRef,
			Bullet:     bulletRef,
			Exists:     exists,
		}
	}
}

func (m *Model) ensurePlaceholderSection(ref events.CollectionRef) bool {
	idx := m.sectionIndexForCollection(ref)
	if idx >= 0 {
		if len(m.sections[idx].Bullets) == 0 && !m.sections[idx].Placeholder {
			m.sections[idx].Placeholder = true
			m.refreshFromSections(false)
			return true
		}
		return false
	}
	id := sectionIDFromRef(ref)
	title := sectionTitleFromRef(ref)
	if strings.TrimSpace(title) == "" {
		title = strings.TrimSpace(ref.Label())
	}
	if title == "" {
		title = "(untitled)"
	}
	m.sections = append(m.sections, Section{
		ID:          id,
		Title:       title,
		Placeholder: true,
	})
	m.rebuildLookup()
	m.refreshFromSections(false)
	return true
}

func (m *Model) applyCollectionChange(msg events.CollectionChangeMsg) bool {
	switch msg.Action {
	case events.ChangeCreate:
		if idx := m.sectionIndexForCollection(msg.Current); idx >= 0 {
			return m.updateSectionFromRef(idx, msg.Current)
		}
		return m.insertSectionFromRef(msg.Current)
	case events.ChangeUpdate:
		if idx := m.sectionIndexForCollection(msg.Current); idx >= 0 {
			return m.updateSectionFromRef(idx, msg.Current)
		}
		if msg.Previous != nil {
			if idx := m.sectionIndexForCollection(*msg.Previous); idx >= 0 {
				return m.updateSectionFromRef(idx, msg.Current)
			}
		}
		return m.insertSectionFromRef(msg.Current)
	case events.ChangeDelete:
		targetIdx := m.sectionIndexForCollection(msg.Current)
		if targetIdx < 0 && msg.Previous != nil {
			targetIdx = m.sectionIndexForCollection(*msg.Previous)
		}
		if targetIdx < 0 {
			return false
		}
		m.sections = append(m.sections[:targetIdx], m.sections[targetIdx+1:]...)
		return true
	default:
		return false
	}
}

func (m *Model) reorderSections(order []string) bool {
	if len(order) == 0 || len(m.sections) <= 1 {
		return false
	}
	index := make(map[string]int, len(order))
	for i, id := range order {
		key := strings.ToLower(strings.TrimSpace(id))
		if key == "" {
			continue
		}
		if _, ok := index[key]; !ok {
			index[key] = i
		}
	}
	before := make([]string, len(m.sections))
	for i, sec := range m.sections {
		before[i] = sectionOrderKey(sec.ID)
	}
	sort.SliceStable(m.sections, func(i, j int) bool {
		return compareSections(m.sections[i], m.sections[j], index)
	})
	for i, sec := range m.sections {
		if before[i] != sectionOrderKey(sec.ID) {
			return true
		}
	}
	return false
}

func (m *Model) insertSectionFromRef(ref events.CollectionRef) bool {
	title := sectionTitleFromRef(ref)
	if title == "" {
		return false
	}
	if idx := m.sectionIndexForCollection(ref); idx >= 0 {
		return false
	}
	m.sections = append(m.sections, Section{
		ID:    sectionIDFromRef(ref),
		Title: title,
	})
	return true
}

func (m *Model) updateSectionFromRef(idx int, ref events.CollectionRef) bool {
	if idx < 0 || idx >= len(m.sections) {
		return false
	}
	changed := false
	id := sectionIDFromRef(ref)
	title := sectionTitleFromRef(ref)
	if id != "" && m.sections[idx].ID != id {
		m.sections[idx].ID = id
		changed = true
	}
	if title != "" && m.sections[idx].Title != title {
		m.sections[idx].Title = title
		changed = true
	}
	return changed
}

func sectionIDFromRef(ref events.CollectionRef) string {
	if ref.ID != "" {
		return strings.TrimSpace(ref.ID)
	}
	return strings.TrimSpace(ref.Label())
}

func sectionTitleFromRef(ref events.CollectionRef) string {
	if ref.Name != "" {
		return strings.TrimSpace(ref.Name)
	}
	if ref.ID != "" {
		return strings.TrimSpace(ref.ID)
	}
	return ""
}

func (m *Model) applyBulletChange(msg events.BulletChangeMsg) bool {
	sectionIdx := m.sectionIndexForView(msg.Collection)
	if sectionIdx < 0 {
		return false
	}
	switch msg.Action {
	case events.ChangeCreate:
		bullet := bulletFromRef(msg.Bullet)
		sec := &m.sections[sectionIdx]
		sec.Bullets = append(sec.Bullets, bullet)
		if len(sec.Bullets) > 0 {
			sec.Placeholder = false
		}
		if strings.TrimSpace(bullet.ID) != "" {
			m.pendingSectionID = sec.ID
			m.pendingBulletID = bullet.ID
		} else {
			m.pendingSectionID = sec.ID
			m.pendingBulletID = ""
		}
		return true
	case events.ChangeUpdate:
		return updateBulletInList(&m.sections[sectionIdx].Bullets, msg.Bullet)
	case events.ChangeDelete:
		return removeBulletFromList(&m.sections[sectionIdx].Bullets, msg.Bullet.ID)
	default:
		return false
	}
}

func bulletFromRef(ref events.BulletRef) Bullet {
	return Bullet{
		ID:        ref.ID,
		Label:     ref.Label,
		Note:      ref.Note,
		Bullet:    ref.Bullet,
		Signifier: ref.Signifier,
	}
}

func updateBulletInList(list *[]Bullet, updated events.BulletRef) bool {
	if updated.ID == "" {
		return false
	}
	if list == nil || len(*list) == 0 {
		return false
	}
	for idx := range *list {
		item := &(*list)[idx]
		if item.ID != "" && item.ID == updated.ID {
			mergeBullet(item, updated)
			return true
		}
		if len(item.Children) > 0 {
			if updateBulletInList(&item.Children, updated) {
				return true
			}
		}
	}
	return false
}

func mergeBullet(dst *Bullet, ref events.BulletRef) {
	if dst == nil {
		return
	}
	dst.Label = ref.Label
	dst.Note = ref.Note
	dst.Bullet = ref.Bullet
	dst.Signifier = ref.Signifier
}

func removeBulletFromList(list *[]Bullet, id string) bool {
	if list == nil || id == "" {
		return false
	}
	items := *list
	for idx := 0; idx < len(items); idx++ {
		item := items[idx]
		if item.ID == id {
			items = append(items[:idx], items[idx+1:]...)
			*list = items
			return true
		}
		if len(item.Children) > 0 {
			if removeBulletFromList(&items[idx].Children, id) {
				*list = items
				return true
			}
		}
	}
	return false
}

func sectionOrderKey(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func compareSections(a, b Section, index map[string]int) bool {
	ai := sectionOrderIndex(a, index)
	bi := sectionOrderIndex(b, index)
	if ai == bi {
		return strings.ToLower(strings.TrimSpace(a.Title)) < strings.ToLower(strings.TrimSpace(b.Title))
	}
	return ai < bi
}

func sectionOrderIndex(sec Section, index map[string]int) int {
	key := sectionOrderKey(sec.ID)
	if pos, ok := index[key]; ok {
		return pos
	}
	return len(index) * 2
}
