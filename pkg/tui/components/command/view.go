package command

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/tui/theme"
	overlaymgr "tableflip.dev/bujo/pkg/tui/ui/overlay"
)

// View renders the combined content, overlay, and command bar.
func (m *Model) View() (string, *tea.Cursor) {
	content := normalizeHeight(m.contentView, m.width, m.contentHeight)

	var contentCursor *tea.Cursor
	if m.contentCursor != nil {
		copy := *m.contentCursor
		contentCursor = &copy
	}

	if m.suggestionOverlay != "" {
		content = overlaymgr.Compose(content, m.width, m.contentHeight, m.suggestionOverlay, m.suggestionPlacement)
	}

	bar, barCursor := m.renderCommandBar()
	if barCursor != nil {
		contentCursor = barCursor
	}

	if content != "" {
		content = content + "\n" + bar
	} else {
		content = bar
	}

	return content, contentCursor
}

func (m *Model) renderCommandBar() (string, *tea.Cursor) {
	var line string
	var cursor *tea.Cursor
	switch m.mode {
	case ModeInput:
		inputView := m.prompt.View()
		line = m.promptPrefix + inputView
		if c := m.prompt.Cursor(); c != nil {
			copy := *c
			copy.X += len(m.promptPrefix)
			copy.Y = m.contentHeight
			cursor = &copy
		}
	default:
		status := m.status
		if status == "" {
			status = "Ready"
		}
		statusStyle := theme.Default().Footer.Status
		value := statusStyle.Render(status)
		available := m.width
		if available < 0 {
			available = 0
		}
		line = lipgloss.NewStyle().Width(available).Align(lipgloss.Right).Render(value)
	}

	line = padToWidth(line, m.width)
	return line, cursor
}

func normalizeHeight(body string, width, height int) string {
	lines := strings.Split(body, "\n")
	if len(lines) > height {
		lines = lines[len(lines)-height:]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	if width > 0 {
		for i := range lines {
			lines[i] = padToWidth(lines[i], width)
		}
	}
	return strings.Join(lines, "\n")
}

func padToWidth(s string, width int) string {
	current := lipgloss.Width(s)
	if current >= width {
		return lipgloss.NewStyle().Width(width).Render(s)
	}
	padding := strings.Repeat(" ", width-current)
	return s + padding
}
