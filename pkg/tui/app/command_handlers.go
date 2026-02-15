package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"

	collectiondetail2 "tableflip.dev/bujo/pkg/tui/components/collectiondetail2"
	journalcomponent "tableflip.dev/bujo/pkg/tui/components/journal"
	"tableflip.dev/bujo/pkg/tui/events"
)

const commandUsageLine = "Commands: :quit, :today, :future, :debug, :report [window], :migrate [window], :details <continuous|focused>, :lock, :unlock, :help"

// commandUsageStatus returns the help text shown when the prompt is empty.
func commandUsageStatus() string {
	return commandUsageLine
}

func (m *Model) handleCommandSubmit(msg events.CommandSubmitMsg) ([]tea.Cmd, bool) {
	if m.command == nil || msg.Component != m.command.ID() {
		return nil, false
	}
	raw := strings.TrimSpace(msg.Value)
	if raw == "" {
		m.setStatus(commandUsageStatus())
		return nil, true
	}
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		m.setStatus(commandUsageStatus())
		return nil, true
	}
	cmdName := strings.ToLower(parts[0])
	arg := ""
	if len(parts) > 1 {
		arg = strings.Join(parts[1:], " ")
	}

	var cmds []tea.Cmd
	switch cmdName {
	case "quit", "exit", "q":
		cmds = append(cmds, tea.Quit)
	case "help":
		cmd, state := m.toggleHelpOverlay()
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		switch state {
		case "opened":
			m.setStatus("Help overlay opened (Esc or : to close)")
		case "closed":
			m.setStatus("Help overlay closed")
		case "noop":
			m.setStatus("Help unavailable")
		}
		m.layoutContent()
	case "debug":
		m.toggleDebug()
	case "report":
		cmd, state := m.showReportOverlay(arg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		switch state {
		case "opened":
			m.setStatus("Report overlay opened")
		case "closed":
			m.setStatus("Report overlay closed")
		case "error":
			// status set inside showReportOverlay
		}
		m.layoutContent()
	case "migrate":
		cmd, state := m.showMigrateOverlay(arg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		switch state {
		case "opened":
			m.setStatus("Migration overlay opened")
		case "closed":
			m.setStatus("Migration overlay closed")
		case "error":
			// status set within showMigrateOverlay
		case "noop":
			// no change
		}
		m.layoutContent()
	case "today":
		if cmd := m.jumpToToday(true); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case "future":
		if cmd := m.jumpToFuture(true); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case "details":
		mode, ok := parseDetailMode(arg)
		if !ok {
			m.setStatus("Usage: :details continuous|focused")
			break
		}
		if m.journalDetail == nil {
			m.setStatus("Detail mode unavailable: journal not ready")
			break
		}
		changed := m.setDetailMode(mode)
		label := "continuous"
		if mode == collectiondetail2.ModeFocused {
			label = "focused"
		}
		if changed {
			m.setStatus("Detail mode: " + label)
		} else {
			m.setStatus("Detail mode already " + label)
		}
		m.layoutContent()
	case "lock":
		if cmd := m.lockSelectedBullet(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case "unlock":
		if cmd := m.unlockSelectedBullet(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	default:
		m.setStatus("Unhandled command: " + cmdName)
	}
	_ = m.dropFocusKind(focusKindCommand)
	return cmds, true
}

func (m *Model) handleCommandCancel(msg events.CommandCancelMsg) ([]tea.Cmd, bool) {
	if m.command == nil || msg.Component != m.command.ID() {
		return nil, false
	}
	m.setStatus("Ready")
	var cmds []tea.Cmd
	if m.commandActive {
		m.commandActive = false
		if !m.helpVisible {
			if cmd := m.focusJournalPane(m.commandReturn); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	_ = m.dropFocusKind(focusKindCommand)
	return cmds, true
}

func (m *Model) handleCommandChange(msg events.CommandChangeMsg) ([]tea.Cmd, bool) {
	if m.command == nil || msg.Component != m.command.ID() {
		_ = m.dropFocusKind(focusKindCommand)
		m.layoutContent()
		return nil, true
	}

	var cmds []tea.Cmd
	if msg.Mode == events.CommandModeInput {
		if !m.commandActive {
			if journal := m.journal(); journal != nil {
				m.commandReturn = journal.FocusedPane()
			} else {
				m.commandReturn = journalcomponent.FocusNav
			}
			m.commandActive = true
			cmds = append(cmds, m.blurJournalPanes()...)
			if m.overlayStack != nil && m.overlayStack.HasOverlay() {
				if cmd := m.overlayStack.Blur(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
			m.pushFocus(focusTarget{kind: focusKindCommand})
		}
	} else {
		if m.commandActive {
			m.commandActive = false
			if m.overlayStack != nil && m.overlayStack.HasOverlay() {
				if cmd := m.overlayStack.Focus(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else if !m.helpVisible {
				if cmd := m.focusJournalPane(m.commandReturn); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
		_ = m.dropFocusKind(focusKindCommand)
	}
	m.layoutContent()
	return cmds, true
}

func parseDetailMode(raw string) (collectiondetail2.Mode, bool) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case "continuous":
		return collectiondetail2.ModeContinuous, true
	case "focused":
		return collectiondetail2.ModeFocused, true
	default:
		return collectiondetail2.ModeContinuous, false
	}
}

func (m *Model) setDetailMode(mode collectiondetail2.Mode) bool {
	changed := m.detailMode != mode
	m.detailMode = mode
	if m.journalDetail != nil {
		if setter, ok := m.journalDetail.(interface {
			SetMode(collectiondetail2.Mode) bool
		}); ok {
			setter.SetMode(mode)
		}
	}
	return changed
}
