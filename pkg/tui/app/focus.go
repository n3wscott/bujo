package app

import (
	tea "github.com/charmbracelet/bubbletea/v2"

	journalcomponent "tableflip.dev/bujo/pkg/tui/components/journal"
)

func (m *Model) restoreFocusAfterOverlay() tea.Cmd {
	var cmds []tea.Cmd
	for {
		target, ok := m.topFocus()
		if !ok {
			break
		}
		if target.kind == focusKindCommand {
			if !m.commandActive {
				_ = m.dropFocusKind(focusKindCommand)
				continue
			}
			break
		}
		if cmd := m.applyFocusTarget(target); cmd != nil {
			cmds = append(cmds, cmd)
		}
		break
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) pushFocus(target focusTarget) {
	if target.kind == focusKindUnknown {
		return
	}
	for i := len(m.focusStack) - 1; i >= 0; i-- {
		if m.focusStack[i].kind == target.kind {
			m.focusStack = m.focusStack[:i]
			break
		}
	}
	m.focusStack = append(m.focusStack, target)
}

func (m *Model) dropFocusKind(kind focusKind) bool {
	for i := len(m.focusStack) - 1; i >= 0; i-- {
		if m.focusStack[i].kind == kind {
			m.focusStack = append(m.focusStack[:i], m.focusStack[i+1:]...)
			return true
		}
	}
	return false
}

func (m *Model) popFocusKind(kind focusKind) (focusTarget, bool) {
	for i := len(m.focusStack) - 1; i >= 0; i-- {
		if m.focusStack[i].kind == kind {
			target := m.focusStack[i]
			m.focusStack = append(m.focusStack[:i], m.focusStack[i+1:]...)
			return target, true
		}
	}
	return focusTarget{}, false
}

func (m *Model) topFocus() (focusTarget, bool) {
	if len(m.focusStack) == 0 {
		return focusTarget{}, false
	}
	return m.focusStack[len(m.focusStack)-1], true
}

func (m *Model) applyFocusTarget(target focusTarget) tea.Cmd {
	switch target.kind {
	case focusKindJournalNav:
		return m.journalFocusCmd(journalcomponent.FocusNav)
	case focusKindJournalDetail:
		return m.journalFocusCmd(journalcomponent.FocusDetail)
	case focusKindCommand:
		if m.command != nil {
			m.command.Focus()
		}
	case focusKindOverlay:
		if m.overlayStack != nil && m.overlayStack.HasOverlay() {
			return m.overlayStack.Focus()
		}
	}
	return nil
}

func (m *Model) blurJournalPanes() []tea.Cmd {
	var cmds []tea.Cmd
	if m.journalNav != nil {
		if cmd := m.journalNav.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.journalDetail != nil {
		if cmd := m.journalDetail.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}

func (m *Model) focusJournalPane(pane journalcomponent.FocusPane) tea.Cmd {
	return m.journalFocusCmd(pane)
}
