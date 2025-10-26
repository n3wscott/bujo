package eventviewer

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/tui/ui"
)

// Level indicates the severity of a logged event.
type Level int

const (
	// LevelInfo is the default severity.
	LevelInfo Level = iota
	// LevelWarn highlights potential issues.
	LevelWarn
	// LevelError highlights failures.
	LevelError
)

// Entry captures a rendered event.
type Entry struct {
	Timestamp time.Time
	Source    string
	Summary   string
	Detail    string
	Level     Level
}

// Model renders a streaming event log.
type Model struct {
	viewport viewport.Model
	entries  []Entry

	maxEntries int
	followTop  bool

	width  int
	height int

	styles Styles
}

// Styles controls the log's presentation.
type Styles struct {
	Frame     lipgloss.Style
	Header    lipgloss.Style
	Info      lipgloss.Style
	Warn      lipgloss.Style
	Error     lipgloss.Style
	Timestamp lipgloss.Style
	Source    lipgloss.Style
}

// DefaultStyles returns the stock styling used by the testbed.
func DefaultStyles() Styles {
	border := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))
	return Styles{
		Frame:     border,
		Header:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("248")),
		Info:      lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Warn:      lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB347")),
		Error:     lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F5F")),
		Timestamp: lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		Source:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
	}
}

// NewModel constructs an event viewer capped at the provided entry count.
func NewModel(maxEntries int) *Model {
	if maxEntries <= 0 {
		maxEntries = 200
	}
	vp := viewport.New(
		viewport.WithWidth(1),
		viewport.WithHeight(1),
	)
	return &Model{
		viewport:   vp,
		maxEntries: maxEntries,
		followTop:  true,
		styles:     DefaultStyles(),
	}
}

// Init implements ui.Component.
func (m *Model) Init() tea.Cmd { return nil }

// Update implements ui.Component. The viewer is passive for now, so this only
// keeps it in sync with the program tick.
func (m *Model) Update(msg tea.Msg) (ui.Component, tea.Cmd) {
	// No-op: the viewport only needs fresh content when entries change.
	return m, nil
}

// SetSize resizes the viewport while keeping the header + border intact.
func (m *Model) SetSize(width, height int) {
	if width < 4 {
		width = 4
	}
	if height < 3 {
		height = 3
	}
	if m.width == width && m.height == height {
		return
	}
	m.width = width
	m.height = height

	innerWidth := max(1, width-2)
	innerHeight := max(1, height-2)
	headerRows := 1
	m.viewport.SetWidth(innerWidth)
	m.viewport.SetHeight(max(1, innerHeight-headerRows))
	m.refreshContent()
}

// View renders the bordered viewport.
func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	header := m.styles.Header.Render("Events")
	body := lipgloss.JoinVertical(lipgloss.Left, header, m.viewport.View())
	return m.styles.Frame.Width(m.width).Height(m.height).Render(body)
}

// Append inserts a new entry at the top of the log.
func (m *Model) Append(entry Entry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if entry.Source == "" {
		entry.Source = "tea"
	}
	if entry.Summary == "" {
		entry.Summary = "event"
	}
	m.entries = append([]Entry{entry}, m.entries...)
	if len(m.entries) > m.maxEntries {
		m.entries = m.entries[:m.maxEntries]
	}
	m.refreshContent()
	if m.followTop {
		m.viewport.SetYOffset(0)
	}
}

// Clear drops all logged entries.
func (m *Model) Clear() {
	m.entries = nil
	m.refreshContent()
}

// WithStyles overrides the default styling.
func (m *Model) WithStyles(styles Styles) {
	m.styles = styles
	m.refreshContent()
}

func (m *Model) refreshContent() {
	lines := make([]string, 0, len(m.entries))
	for _, entry := range m.entries {
		lines = append(lines, m.renderEntry(entry))
	}
	content := strings.Join(lines, "\n")
	if content == "" {
		content = m.styles.Timestamp.Render("No events yet")
	}
	m.viewport.SetContent(content)
}

func (m *Model) renderEntry(entry Entry) string {
	ts := m.styles.Timestamp.Render(entry.Timestamp.Format("15:04:05.000"))
	source := m.styles.Source.Render(fmt.Sprintf("[%s]", entry.Source))
	msg := entry.Summary
	if entry.Detail != "" {
		msg = fmt.Sprintf("%s — %s", msg, entry.Detail)
	}
	switch entry.Level {
	case LevelWarn:
		msg = m.styles.Warn.Render(msg)
	case LevelError:
		msg = m.styles.Error.Render(msg)
	default:
		msg = m.styles.Info.Render(msg)
	}
	return fmt.Sprintf("%s %s %s", ts, source, msg)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
