package app

import (
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/timeutil"
	"tableflip.dev/bujo/pkg/tui/components/command"
)

func (m *Model) showReportOverlay(arg string) (tea.Cmd, string) {
	if m.command == nil {
		return nil, "noop"
	}
	m.ensureOverlayStack()
	if m.service == nil {
		m.setStatus("Report unavailable: service offline")
		return nil, "error"
	}
	if m.reportVisible {
		return m.closeReportOverlay(), "closed"
	}
	dur, label, err := m.parseReportWindow(arg)
	if err != nil {
		m.setStatus("Report: " + err.Error())
		return nil, "error"
	}
	var cmds []tea.Cmd
	if m.helpVisible {
		if cmd := m.closeHelpOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.addVisible {
		if cmd := m.closeAddTaskOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.newCollectionVisible {
		if cmd := m.closeNewCollectionOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	overlay := newReportOverlay(m.service, dur, label)
	placement := m.reportPlacement()
	width := placement.Width
	if width <= 0 {
		width = m.width
	}
	height := placement.Height
	if height <= 0 {
		height = maxInt(10, m.height-1)
	}
	m.report = overlay
	overlay.SetSize(width, height)
	cmd := m.overlayStack.Open(overlayKindReport, overlay, placement)
	m.reportVisible = true
	_ = m.dropFocusKind(focusKindCommand)
	m.pushFocus(focusTarget{kind: focusKindOverlay, overlay: overlayKindReport})
	cmds = append(cmds, m.blurJournalPanes()...)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	if focusCmd := m.overlayStack.Focus(); focusCmd != nil {
		cmds = append(cmds, focusCmd)
	}
	if len(cmds) == 0 {
		return nil, "opened"
	}
	return tea.Batch(cmds...), "opened"
}

func (m *Model) reportPlacement() command.OverlayPlacement {
	availableWidth := m.width
	if availableWidth <= 0 {
		availableWidth = 1
	}
	width := int(math.Round(float64(availableWidth) * 0.9))
	if width <= 0 || width > availableWidth {
		width = availableWidth
	}
	if width < 20 {
		width = minInt(20, availableWidth)
	}
	availableHeight := m.height - 1
	if availableHeight <= 0 {
		availableHeight = 1
	}
	height := int(math.Round(float64(availableHeight) * 0.9))
	if height <= 0 || height > availableHeight {
		height = availableHeight
	}
	if height < 5 {
		height = minInt(availableHeight, 5)
	}
	return command.OverlayPlacement{
		Width:      width,
		Height:     height,
		Horizontal: lipgloss.Center,
		Vertical:   lipgloss.Top,
	}
}

func (m *Model) parseReportWindow(spec string) (time.Duration, string, error) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return timeutil.ParseWindow(timeutil.DefaultWindow)
	}
	normalized := strings.ReplaceAll(trimmed, " ", "")
	return timeutil.ParseWindow(normalized)
}

func (m *Model) closeReportOverlay() tea.Cmd {
	if !m.reportVisible {
		return nil
	}
	var cmds []tea.Cmd
	if m.overlayStack != nil {
		if cmd := m.overlayStack.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.overlayStack.Close(overlayKindReport)
	}
	m.reportVisible = false
	m.report = nil
	_, _ = m.popFocusKind(focusKindOverlay)
	if cmd := m.restoreFocusAfterOverlay(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if m.command != nil {
		m.setStatusIfIdle("Report overlay closed")
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}
