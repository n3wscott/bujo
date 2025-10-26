package collectiondetail

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/muesli/reflow/wordwrap"
)

// Bullet describes a single entry row inside a collection detail section.
type Bullet struct {
	ID    string
	Label string
	Note  string
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

	lines       []lineInfo
	bulletLines []int
}

const (
	lineHeader = -1
	lineSpacer = -2
	lineEmpty  = -3
)

type lineInfo struct {
	section int
	kind    int // >=0 bullet index, otherwise line constants above
}

// NewModel constructs the detail component with the provided sections.
func NewModel(sections []Section) *Model {
	m := &Model{cursor: -1}
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
func (m *Model) Focus() {
	m.focused = true
}

// Blur marks the component as inactive.
func (m *Model) Blur() {
	m.focused = false
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// Update handles key presses for navigation.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
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
		}
	}
	return m, nil
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
	start := m.scroll
	end := m.scroll + m.height
	if end > len(m.lines) {
		end = len(m.lines)
	}
	activeLine := m.currentLineIndex()
	for i := start; i < end; i++ {
		if i > start {
			b.WriteByte('\n')
		}
		b.WriteString(m.renderLine(i, i == activeLine))
	}
	return b.String()
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
			for bi := range sec.Bullets {
				m.lines = append(m.lines, lineInfo{section: si, kind: bi})
				m.bulletLines = append(m.bulletLines, len(m.lines)-1)
			}
		}
		// Spacer line between sections (except after final section we trim below).
		m.lines = append(m.lines, lineInfo{section: si, kind: lineSpacer})
	}
	if len(m.lines) > 0 {
		// Remove trailing spacer.
		m.lines = m.lines[:len(m.lines)-1]
	}
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
	default:
		return m.renderBullet(info.section, info.kind, selected)
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
		title = title + " · " + sec.Subtitle
	}
	return style.Width(m.width).Render(title)
}

func (m *Model) renderBullet(section, bullet int, selected bool) string {
	sec := m.sections[section]
	if bullet < 0 || bullet >= len(sec.Bullets) {
		return ""
	}
	item := sec.Bullets[bullet]
	prefix := "  - "
	if selected && m.focused {
		prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Render("  → ")
	} else if selected {
		prefix = "  → "
	}
	text := item.Label
	if strings.TrimSpace(text) == "" {
		text = "<empty>"
	}
	if item.Note != "" {
		text = text + " · " + item.Note
	}
	lines := m.wrapBulletLines(prefix, text)
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
