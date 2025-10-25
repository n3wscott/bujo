// Package panel defines reusable overlay panel models for the TUI.
package panel

import (
	"strings"

	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/tui/theme"
)

// Model renders a generic information panel with a title and body lines.
type Model struct {
	title      string
	lines      []string
	frameStyle lipgloss.Style
	titleStyle lipgloss.Style
	bodyStyle  lipgloss.Style
}

// New returns a panel model with sensible defaults.
func New(th theme.PanelTheme) Model {
	return Model{
		frameStyle: th.Frame,
		titleStyle: th.Title,
		bodyStyle:  th.Body,
	}
}

// SetContent updates the panel title and body lines.
func (m *Model) SetContent(title string, lines []string) {
	m.title = title
	m.lines = lines
}

// Reset clears panel content.
func (m *Model) Reset() {
	m.title = ""
	m.lines = nil
}

// View returns the rendered panel string and its total height in lines.
func (m Model) View() (string, int) {
	var content []string
	if m.title != "" {
		content = append(content, m.titleStyle.Render(m.title))
	}
	for _, line := range m.lines {
		content = append(content, m.bodyStyle.Render(line))
	}
	view := m.frameStyle.Render(strings.Join(content, "\n"))
	height := strings.Count(view, "\n") + 1
	return view, height
}
