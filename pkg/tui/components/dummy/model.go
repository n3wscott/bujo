package dummy

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/tui/components/command"
)

// Model renders placeholder lines showing the current size.
type Model struct {
	width  int
	height int

	frame   lipgloss.Style
	content []string
}

// New constructs a dummy component sized to width x height.
func New(width, height int) *Model {
	m := &Model{
		frame: lipgloss.NewStyle().
			Margin(0).
			Padding(0),
	}
	m.SetSize(width, height)
	return m
}

// Init implements command.Overlay.
func (m *Model) Init() tea.Cmd { return nil }

// Update implements command.Overlay.
func (m *Model) Update(msg tea.Msg) (command.Overlay, tea.Cmd) { return m, nil }

// View renders the placeholder content.
func (m *Model) View() (string, *tea.Cursor) {
	body := strings.Join(m.content, "\n")
	return m.frame.Width(m.width).Height(m.height).Render(body), nil
}

// SetSize updates dimensions and rebuilds placeholder lines.
func (m *Model) SetSize(width, height int) {
	if width < 2 {
		width = 2
	}
	if height < 1 {
		height = 1
	}
	if m.width == width && m.height == height {
		return
	}
	m.width = width
	m.height = height
	m.content = make([]string, height)
	for i := 0; i < height; i++ {
		m.content[i] = buildLine(i+1, width)
	}
}

func buildLine(lineNumber, width int) string {
	if width < 2 {
		return "S"
	}
	if width == 2 {
		return "SE"
	}
	limit := width - 1 // reserve trailing E
	num := strconv.Itoa(lineNumber)
	var b strings.Builder
	b.Grow(width)
	b.WriteByte('S')
	pos := 1
	for pos+len(num)+1 <= width {
		b.WriteByte(' ')
		b.WriteString(num)
		pos += len(num) + 1
	}
	for pos < limit {
		b.WriteByte(' ')
		pos++
	}
	if b.Len() > width-1 {
		line := b.String()[:width-1]
		return line + "E"
	}
	return b.String() + "E"
}
