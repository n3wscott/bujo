package collectiondetail

import (
	"fmt"
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
	ID       string
	Title    string
	Subtitle string
	Bullets  []Bullet
}

// Model renders a scrollable list of section headers with their bullets.
type Model struct {
	sections []Section

	width  int
	height int

	cursor int // index into bulletLines, -1 when nothing selectable
	scroll int

	focused bool

	lines         []lineInfo
	bulletLines   []int
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
	m := &Model{cursor: -1, id: events.ComponentID("collectiondetail")}
	m.SetSections(sections)
	return m
}

// SetSections replaces the rendered sections.
func (m *Model) SetSections(sections []Section) {
	m.sections = append([]Section(nil), sections...)
	m.rebuildLines()
	if len(m.bulletLines) == 0 {
		m.cursor = -1
	} else if m.cursor < 0 || m.cursor >= len(m.bulletLines) {
		m.cursor = 0
	}
	m.ensureScroll()
	m.lastHighlight = ""
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
			}
		case "end", "G":
			if len(m.bulletLines) > 0 {
				m.cursor = len(m.bulletLines) - 1
				m.ensureScroll()
			}
		case "enter", " ":
			if cmd := m.selectCmd(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	case events.CollectionHighlightMsg:
		if m.sourceNav == "" || m.sourceNav == msg.Component {
			if m.focusSectionForCollection(msg.Collection) {
				// no cmd, but we updated scroll/cursor
			}
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
	var b strings.Builder

	stickySection, stickyTitle, hasSticky := m.visibleSection()
	contentHeight := m.height
	if hasSticky {
		title := m.renderStickyTitle(stickyTitle, m.sectionActive(stickySection))
		titleHeight := lipgloss.Height(title)
		if contentHeight < titleHeight {
			contentHeight = 0
		} else {
			contentHeight -= titleHeight
		}
		b.WriteString(title)
		if contentHeight > 0 {
			b.WriteByte('\n')
		}
	}
	if contentHeight <= 0 {
		contentHeight = 1
	}

	start := m.scroll
	end := m.scroll + m.height
	if end > len(m.lines) {
		end = len(m.lines)
	}
	activeLine := m.currentLineIndex()
	skippedHeader := hasSticky
	lineWritten := false
	for i := start; i < end; i++ {
		info := m.lines[i]
		if hasSticky && skippedHeader && info.kind == lineHeader && info.section == stickySection {
			skippedHeader = false
			continue
		}
		if lineWritten {
			b.WriteByte('\n')
		}
		b.WriteString(m.renderLine(i, i == activeLine))
		lineWritten = true
	}

	return b.String()
}

func (m *Model) visibleSection() (int, string, bool) {
	if len(m.lines) == 0 || len(m.sections) == 0 {
		return -1, "", false
	}
	start := m.scroll
	end := m.scroll + m.height
	if end > len(m.lines) {
		end = len(m.lines)
	}
	for i := start; i < end; i++ {
		info := m.lines[i]
		if info.section < 0 || info.section >= len(m.sections) {
			continue
		}
		if info.kind != lineItem {
			continue
		}
		title := m.sections[info.section].Title
		if strings.TrimSpace(title) == "" {
			title = "(untitled)"
		}
		return info.section, title, true
	}
	return -1, "", false
}

func (m *Model) renderStickyTitle(title string, highlight bool) string {
	style := lipgloss.NewStyle().Bold(true)
	if highlight && m.focused {
		style = style.Foreground(lipgloss.Color("213"))
	}
	return style.Width(m.width).Render(title)
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
}

func (m *Model) ensureScroll() {
	if m.height <= 0 || len(m.lines) == 0 {
		return
	}
	curLine := m.currentLineIndex()
	if curLine < 0 {
		m.scroll = 0
		return
	}
	if curLine < m.scroll {
		m.scroll = curLine
		return
	}
	viewBottom := m.scroll + m.height - 1
	if curLine > viewBottom {
		m.scroll = curLine - m.height + 1
		if m.scroll < 0 {
			m.scroll = 0
		}
	}
	maxScroll := len(m.lines) - m.height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scroll > maxScroll {
		m.scroll = maxScroll
	}
}

func (m *Model) pageSize() int {
	if m.height <= 0 {
		return 10
	}
	return m.height - 1
}

func (m *Model) rebuildLines() {
	m.lines = m.lines[:0]
	m.bulletLines = m.bulletLines[:0]
	for si, sec := range m.sections {
		m.lines = append(m.lines, lineInfo{section: si, kind: lineHeader})
		if len(sec.Bullets) == 0 {
			m.lines = append(m.lines, lineInfo{section: si, kind: lineEmpty})
		} else {
			m.appendBulletLines(si, sec.Bullets, 0)
		}
		m.lines = append(m.lines, lineInfo{section: si, kind: lineSpacer})
	}
	if len(m.lines) > 0 {
		m.lines = m.lines[:len(m.lines)-1]
	}
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
		return "  <empty>"
	case lineItem:
		return m.renderBulletInfo(info, selected)
	default:
		return ""
	}
}

func (m *Model) renderSectionHeader(section int, highlight bool) string {
	sec := m.sections[section]
	style := lipgloss.NewStyle().Bold(true)
	if highlight && m.focused {
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

func (m *Model) renderBulletPrefix(selected bool) string {
	base := " "
	arrow := "-"
	if selected && m.focused {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Render("  →")
	} else if selected {
		return "  →"
	}
	return base + arrow + " "
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

func (m *Model) currentLineIndex() int {
	if m.cursor < 0 || m.cursor >= len(m.bulletLines) {
		return -1
	}
	return m.bulletLines[m.cursor]
}

func (m *Model) sectionActive(section int) bool {
	lineIdx := m.currentLineIndex()
	if lineIdx < 0 || lineIdx >= len(m.lines) {
		return false
	}
	return m.lines[lineIdx].section == section
}

func (m *Model) sectionIndexForCollection(ref events.CollectionRef) int {
	if len(m.sections) == 0 {
		return -1
	}
	for idx, sec := range m.sections {
		if sec.ID != "" && ref.ID != "" && strings.EqualFold(sec.ID, ref.ID) {
			return idx
		}
		if sec.Title != "" && ref.Name != "" && strings.EqualFold(sec.Title, ref.Name) {
			return idx
		}
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
		return true
	}
	m.scrollToLine(targetLine)
	return true
}

func (m *Model) scrollToLine(line int) {
	if m.height <= 0 {
		return
	}
	if line < 0 || line >= len(m.lines) {
		return
	}
	if line < m.scroll {
		m.scroll = line
		return
	}
	viewBottom := m.scroll + m.height - 1
	if line > viewBottom {
		m.scroll = line - m.height + 1
		if m.scroll < 0 {
			m.scroll = 0
		}
	}
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

func (m *Model) highlightCmd() tea.Cmd {
	info, section, ok := m.currentBulletInfo()
	if !ok {
		if m.lastHighlight != "" {
			m.lastHighlight = ""
		}
		return nil
	}
	key := highlightKey(info)
	if key == m.lastHighlight {
		return nil
	}
	m.lastHighlight = key
	return bulletHighlightCmd(m.id, section, info.bullet)
}

func (m *Model) selectCmd() tea.Cmd {
	info, section, ok := m.currentBulletInfo()
	if !ok {
		return nil
	}
	return bulletSelectCmd(m.id, section, info.bullet)
}

func highlightKey(info lineInfo) string {
	if info.bullet.ID != "" {
		return info.bullet.ID
	}
	return fmt.Sprintf("%d:%s", info.section, info.bullet.Label)
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
