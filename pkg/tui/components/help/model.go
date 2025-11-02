package help

import (
	_ "embed"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/tui/components/command"
)

//go:embed help.md
var helpMarkdown string

// Model renders the Glamour-based help overlay inside a bordered viewport.
type Model struct {
	viewport viewport.Model
	width    int
	height   int

	frame lipgloss.Style
	err   error
}

// New constructs a help overlay model sized to the provided bounds.
func New(width, height int) *Model {
	vp := viewport.New(
		viewport.WithWidth(max(width, 1)),
		viewport.WithHeight(max(height, 1)),
	)
	vp.MouseWheelEnabled = true
	frame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Margin(0).
		Padding(0)
	model := &Model{
		viewport: vp,
		frame:    frame,
	}
	model.SetSize(width, height)
	return model
}

// Init implements command.Overlay.
func (m *Model) Init() tea.Cmd { return nil }

// Update handles Bubble Tea messages and forwards scrolling to the viewport.
func (m *Model) Update(msg tea.Msg) (command.Overlay, tea.Cmd) {
	vp, cmd := m.viewport.Update(msg)
	m.viewport = vp
	return m, cmd
}

// View renders the help content inside a rounded frame.
func (m *Model) View() (string, *tea.Cursor) {
	body := m.viewport.View()
	if body == "" && m.err != nil {
		body = "help unavailable: " + m.err.Error()
	}
	return m.frame.Width(m.width).Height(m.height).Render(body), nil
}

// SetSize configures the overlay dimensions and re-renders the markdown to fit.
func (m *Model) SetSize(width, height int) {
	minWidth, minHeight := 32, 8
	if width < minWidth {
		width = minWidth
	}
	if height < minHeight {
		height = minHeight
	}
	if m.width == width && m.height == height {
		return
	}

	m.width = width
	m.height = height

	frameX := m.frame.GetHorizontalFrameSize()
	frameY := m.frame.GetVerticalFrameSize()

	innerWidth := max(width-frameX, 1)
	innerHeight := max(height-frameY, 1)

	m.viewport.SetWidth(innerWidth)
	m.viewport.SetHeight(innerHeight)

	m.renderContent(innerWidth)
}

func (m *Model) renderContent(wrap int) {
	renderWidth := max(wrap, 10)
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(renderWidth),
	)
	if err != nil {
		m.err = err
		m.viewport.SetContent("help unavailable: " + err.Error())
		return
	}

	content, err := renderer.Render(strings.TrimSpace(helpMarkdown))
	if err != nil {
		m.err = err
		m.viewport.SetContent("help unavailable: " + err.Error())
		return
	}

	content = stripANSI(content)

	m.err = nil
	m.viewport.SetContent(content)
	m.viewport.SetYOffset(0)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;:]*[A-Za-z~]`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}
