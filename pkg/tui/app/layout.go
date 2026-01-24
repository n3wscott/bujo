package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/tui/components/eventviewer"
)

// View renders the composed UI.
func (m *Model) View() (string, *tea.Cursor) {
	if m.command == nil {
		return "initializing…", nil
	}
	return m.command.View()
}

func (m *Model) layoutContent() {
	if m.command == nil {
		return
	}
	if m.width <= 0 {
		m.width = 1
	}
	if m.height <= 0 {
		m.height = 1
	}

	m.command.SetSize(m.width, m.height)

	totalRows := maxInt(1, m.height-1)
	debugRows := 0
	if m.debugEnabled {
		if m.eventViewer == nil {
			m.eventViewer = eventviewer.NewModel(400)
		}
		debugRows = m.computeDebugHeight(totalRows)
		if debugRows > 0 {
			m.eventViewer.SetSize(m.width, debugRows)
		}
	} else {
		m.eventViewer = nil
	}

	mainRows := totalRows
	if debugRows > 0 && debugRows < totalRows {
		mainRows = totalRows - debugRows
	}
	if mainRows < 1 {
		mainRows = 1
	}
	mainView, mainCursor := m.mainContent(mainRows)
	if m.overlayStack == nil {
		m.overlayStack = newOverlayStack(m.width, mainRows)
	}
	m.overlayStack.SetSize(m.width, mainRows)
	m.overlayStack.SetBackground(mainView, mainCursor)
	composed, composedCursor := m.overlayStack.View()
	body := composed
	if debugRows > 0 && m.eventViewer != nil {
		debugView := m.eventViewer.View()
		if body != "" {
			body = body + "\n" + debugView
		} else {
			body = debugView
		}
	}
	m.command.SetContent(body, composedCursor)
}

func (m *Model) mainContent(height int) (string, *tea.Cursor) {
	if m.router != nil {
		if view, cursor, ok := m.router.ActiveView(m.width, height); ok {
			if height < 1 {
				height = 1
			}
			viewLines := strings.Split(view, "\n")
			if len(viewLines) > 0 && viewLines[len(viewLines)-1] == "" {
				viewLines = viewLines[:len(viewLines)-1]
			}
			if len(viewLines) > height {
				viewLines = viewLines[:height]
			}
			for len(viewLines) < height {
				viewLines = append(viewLines, "")
			}
			body := strings.Join(viewLines, "\n")
			return body, cursor
		}
	}

	var lines []string
	if m.loadingJournal {
		lines = append(lines, m.clipLine("Loading journal…"))
	} else if m.journalError != nil {
		lines = append(lines, m.clipLine("Journal load failed: "+m.journalError.Error()))
	} else {
		lines = append(lines, m.clipLine("Journal not available"))
	}
	return strings.Join(lines, "\n"), nil
}

func (m *Model) clipLine(text string) string {
	if m.width <= 0 {
		return text
	}
	if len(text) <= m.width {
		return text
	}
	if m.width <= 3 {
		return text[:m.width]
	}
	return text[:m.width-3] + "..."
}

func (m *Model) computeDebugHeight(totalRows int) int {
	if totalRows <= 4 {
		return 0
	}
	minHeight := 5
	maxHeight := totalRows - 1
	if maxHeight < minHeight {
		return maxHeight
	}
	desired := clamp(totalRows/3, minHeight, minInt(12, maxHeight))
	return desired
}
