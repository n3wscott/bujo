package app

import (
	"math"
	"os"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/tui/components/command"
	dummyview "tableflip.dev/bujo/pkg/tui/components/dummy"
	helpview "tableflip.dev/bujo/pkg/tui/components/help"
	journalcomponent "tableflip.dev/bujo/pkg/tui/components/journal"
)

func (m *Model) toggleHelpOverlay() (tea.Cmd, string) {
	if m.command == nil {
		return nil, "noop"
	}
	m.ensureOverlayStack()
	if m.helpVisible {
		return m.closeHelpOverlay(), "closed"
	}
	cmd := m.openHelpOverlay()
	if cmd == nil {
		return nil, "opened"
	}
	return cmd, "opened"
}

func (m *Model) helpPlacement() command.OverlayPlacement {
	availableWidth := m.width
	if availableWidth <= 0 {
		availableWidth = 1
	}
	width := int(math.Round(float64(availableWidth) * 0.75))
	if width <= 0 || width > availableWidth {
		width = availableWidth
	}
	if width < 38 {
		width = minInt(availableWidth, 38)
	}
	availableHeight := m.height - 1
	if availableHeight <= 0 {
		availableHeight = 1
	}
	height := int(math.Round(float64(availableHeight) * 0.8))
	if height <= 0 || height > availableHeight {
		height = availableHeight
	}
	if height < 10 {
		height = minInt(availableHeight, 10)
	}
	return command.OverlayPlacement{
		Width:      width,
		Height:     height,
		Horizontal: lipgloss.Center,
		Vertical:   lipgloss.Top,
	}
}

func (m *Model) openHelpOverlay() tea.Cmd {
	m.ensureOverlayStack()
	var cmds []tea.Cmd
	if m.addVisible {
		if cmd := m.closeAddTaskOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.detailVisible {
		if cmd := m.closeBulletDetailOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.newCollectionVisible {
		if cmd := m.closeNewCollectionOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	placement := m.helpPlacement()
	width := placement.Width
	if width <= 0 {
		width = m.width
	}
	height := placement.Height
	if height <= 0 {
		height = maxInt(10, m.height-1)
	}
	var overlay command.Overlay
	if os.Getenv("BUJO_HELP_DUMMY") == "1" {
		overlay = dummyview.New(width, height)
	} else {
		overlay = helpview.New(width, height)
	}
	overlay.SetSize(width, height)

	if m.reportVisible {
		if cmd := m.closeReportOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if journal := m.journal(); journal != nil {
		m.helpReturn = journal.FocusedPane()
	} else {
		m.helpReturn = journalcomponent.FocusNav
	}
	m.helpHadFocus = true
	cmds = append(cmds, m.blurJournalPanes()...)
	if cmd := m.overlayStack.Open(overlayKindHelp, overlay, placement); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if focusCmd := m.overlayStack.Focus(); focusCmd != nil {
		cmds = append(cmds, focusCmd)
	}
	m.helpVisible = true
	_ = m.dropFocusKind(focusKindCommand)
	m.pushFocus(focusTarget{kind: focusKindOverlay, overlay: overlayKindHelp})
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) closeHelpOverlay() tea.Cmd {
	if !m.helpVisible {
		return nil
	}
	var cmds []tea.Cmd
	if m.overlayStack != nil {
		if cmd := m.overlayStack.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.overlayStack.Close(overlayKindHelp)
	}
	m.helpVisible = false
	m.helpHadFocus = false
	_, _ = m.popFocusKind(focusKindOverlay)
	if cmd := m.restoreFocusAfterOverlay(); cmd != nil {
		cmds = append(cmds, cmd)
	} else {
		if restore := m.journalFocusCmd(m.helpReturn); restore != nil {
			cmds = append(cmds, restore)
		}
	}
	if m.command != nil {
		m.setStatusIfIdle("Help overlay closed")
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}
