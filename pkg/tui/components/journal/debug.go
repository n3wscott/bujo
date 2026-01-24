package journal

import (
	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/tui/events"
)

// debugCmd emits a debug event when journal routing needs to log context.
func (m *Model) debugCmd(context, detail string) tea.Cmd {
	if context == "" && detail == "" {
		return nil
	}
	return events.DebugCmd(m.id, context, detail)
}
