package journal

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/tui/events"
)

// handleSelectionEvent routes selection/highlight events between nav and detail panes.
func (m *Model) handleSelectionEvent(msg tea.Msg) []tea.Cmd {
	var cmds []tea.Cmd
	switch evt := msg.(type) {
	case events.BulletHighlightMsg:
		if m.detailID != "" && evt.Component == m.detailID && m.nav != nil {
			ref := events.CollectionRef{ID: evt.Collection.ID, Name: evt.Collection.Title}
			cmds = appendCmd(cmds, m.debugCmd("bullet-highlight", fmt.Sprintf("select nav collection %s", ref.Label())))
			if cmd := m.nav.SelectCollection(ref); cmd != nil {
				cmds = appendCmd(cmds, cmd)
			}
		}
	case events.BulletSelectMsg:
		if evt.Component == m.detailID {
			if cmd := events.BulletDetailRequestCmd(m.id, evt.Collection, evt.Bullet); cmd != nil {
				cmds = appendCmd(cmds, cmd)
			}
		}
	case events.CollectionSelectMsg:
		if evt.Component == m.navID {
			cmds = appendCmd(cmds, m.debugCmd("collection-select", fmt.Sprintf("focus detail for %s", evt.Collection.Label())))
			if cmd := m.FocusDetail(); cmd != nil {
				cmds = appendCmd(cmds, cmd)
			}
		}
	}
	return cmds
}
