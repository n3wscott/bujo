package command

import (
	"strings"

	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/tui/theme"
	overlaymgr "tableflip.dev/bujo/pkg/tui/ui/overlay"
)

func (m *Model) applySuggestionFilter(value string, resetSelection bool) {
	if m.mode != ModeInput {
		m.filteredSuggestions = nil
		m.suggestionOverlay = ""
		m.suggestionWindowStart = 0
		return
	}

	prefix := strings.TrimSpace(strings.ToLower(value))
	matches := make([]SuggestionOption, 0, len(m.suggestions))
	if prefix == "" {
		matches = append(matches, m.suggestions...)
	} else {
		seen := make(map[string]struct{}, len(m.suggestions))
		for _, opt := range m.suggestions {
			name := strings.ToLower(opt.Name)
			if strings.HasPrefix(name, prefix) {
				matches = append(matches, opt)
				seen[opt.Name] = struct{}{}
			}
		}
		for _, opt := range m.suggestions {
			if _, ok := seen[opt.Name]; ok {
				continue
			}
			name := strings.ToLower(opt.Name)
			if strings.Contains(name, prefix) {
				matches = append(matches, opt)
			}
		}
	}

	if cap(m.filteredSuggestions) < len(matches) {
		m.filteredSuggestions = make([]SuggestionOption, 0, len(matches))
	}
	m.filteredSuggestions = m.filteredSuggestions[:0]
	m.filteredSuggestions = append(m.filteredSuggestions, matches...)

	if resetSelection {
		m.suggestionIndex = -1
		m.suggestionWindowStart = 0
		m.suggestionOriginal = value
	} else {
		if m.suggestionIndex >= len(m.filteredSuggestions) {
			m.suggestionIndex = len(m.filteredSuggestions) - 1
		}
		if m.suggestionIndex < -1 {
			m.suggestionIndex = -1
		}
	}

	m.updateSuggestionWindow()
	m.refreshSuggestionOverlay()
}

func (m *Model) effectiveSuggestionLimit() int {
	total := len(m.filteredSuggestions)
	if total == 0 {
		return 0
	}
	limit := m.suggestionLimit
	if limit <= 0 || limit > total {
		limit = total
	}
	maxRows := m.height - 1
	if maxRows < 0 {
		maxRows = 0
	}
	if limit > maxRows {
		limit = maxRows
	}
	if limit < 0 {
		limit = 0
	}
	return limit
}

func (m *Model) updateSuggestionWindow() {
	total := len(m.filteredSuggestions)
	if total == 0 {
		m.suggestionWindowStart = 0
		return
	}

	limit := m.effectiveSuggestionLimit()
	if limit <= 0 {
		m.suggestionWindowStart = 0
		return
	}

	if m.suggestionWindowStart > total-limit {
		m.suggestionWindowStart = total - limit
	}
	if m.suggestionWindowStart < 0 {
		m.suggestionWindowStart = 0
	}

	if m.suggestionIndex >= 0 {
		if m.suggestionIndex < m.suggestionWindowStart {
			m.suggestionWindowStart = m.suggestionIndex
		} else if m.suggestionIndex >= m.suggestionWindowStart+limit {
			m.suggestionWindowStart = m.suggestionIndex - limit + 1
		}
	}
}

func (m *Model) refreshSuggestionOverlay() {
	if m.mode != ModeInput || len(m.filteredSuggestions) == 0 {
		m.suggestionOverlay = ""
		return
	}
	limit := m.effectiveSuggestionLimit()
	if limit <= 0 {
		m.suggestionOverlay = ""
		return
	}
	start := m.suggestionWindowStart
	if start < 0 {
		start = 0
	}
	maxStart := len(m.filteredSuggestions) - limit
	if maxStart < 0 {
		maxStart = 0
	}
	if start > maxStart {
		start = maxStart
	}
	end := start + limit
	if end > len(m.filteredSuggestions) {
		end = len(m.filteredSuggestions)
	}

	maxWidth := 0
	count := end - start
	if count <= 0 {
		m.suggestionOverlay = ""
		return
	}
	rows := make([]string, count)
	footerTheme := theme.Default().Footer
	nameStyle := footerTheme.CommandName
	descStyle := footerTheme.CommandDescription
	primaryStyle := footerTheme.CommandSelectedName
	primaryDesc := footerTheme.CommandSelectedDesc

	for i := start; i < end; i++ {
		opt := m.filteredSuggestions[i]
		marker := "  "
		nameRender := nameStyle.Render(opt.Name)
		descRender := descStyle.Render(strings.TrimSpace(opt.Description))
		if i == m.suggestionIndex {
			marker = "→ "
			nameRender = primaryStyle.Render(opt.Name)
			descRender = primaryDesc.Render(strings.TrimSpace(opt.Description))
		}
		line := marker + strings.TrimSpace(nameRender)
		if dr := strings.TrimSpace(descRender); dr != "" {
			line += "  " + dr
		}
		rowIdx := i - start
		rows[rowIdx] = line
		if w := lipgloss.Width(line); w > maxWidth {
			maxWidth = w
		}
	}

	availableWidth := m.width
	if availableWidth <= 0 {
		availableWidth = 10
	}
	if maxWidth > availableWidth {
		maxWidth = availableWidth
	}
	if maxWidth <= 0 {
		maxWidth = availableWidth
	}

	padding := 2
	maxWidthWithPadding := maxWidth + padding
	if maxWidthWithPadding > availableWidth {
		maxWidthWithPadding = availableWidth
		if maxWidthWithPadding > maxWidth {
			padding = maxWidthWithPadding - maxWidth
		} else {
			padding = 0
		}
	}
	// TODO: there is an interaction between the collection detail viewport scroll
	// and the command suggestions overlay that causes the overlay to jump; render
	// the overlay full width for now while we investigate a tighter fix.
	styleWidth := m.width
	if styleWidth <= 0 {
		styleWidth = maxWidthWithPadding
	}
	contentStyle := lipgloss.NewStyle().Width(styleWidth).Align(lipgloss.Left)
	for i := range rows {
		if padding > 0 {
			rows[i] += strings.Repeat(" ", padding)
		}
		rows[i] = contentStyle.Render(rows[i])
	}

	m.suggestionOverlay = strings.Join(rows, "\n")
	height := strings.Count(m.suggestionOverlay, "\n") + 1
	if height <= 0 {
		height = len(rows)
		if height <= 0 {
			height = 1
		}
	}
	placementWidth := styleWidth
	m.suggestionPlacement = overlaymgr.Placement{
		Horizontal: overlayAlignLeft,
		Vertical:   lipgloss.Bottom,
		MarginX:    0,
		MarginY:    0,
		Width:      placementWidth,
		Height:     height,
	}
}

func (m *Model) cycleSuggestion(delta int) bool {
	if m.mode != ModeInput {
		return false
	}
	total := len(m.filteredSuggestions)
	if total == 0 {
		return false
	}
	if m.effectiveSuggestionLimit() == 0 {
		return false
	}
	if m.suggestionIndex == -1 {
		if delta > 0 {
			m.suggestionIndex = 0
		} else {
			m.suggestionIndex = total - 1
		}
		m.suggestionOriginal = m.prompt.Value()
	} else {
		m.suggestionIndex = (m.suggestionIndex + delta) % total
		if m.suggestionIndex < 0 {
			m.suggestionIndex += total
		}
	}
	if m.suggestionIndex < 0 || m.suggestionIndex >= total {
		m.clearSuggestionSelection()
		return false
	}
	choice := m.filteredSuggestions[m.suggestionIndex]
	m.prompt.SetValue(choice.Name)
	m.prompt.CursorEnd()
	m.updateSuggestionWindow()
	m.refreshSuggestionOverlay()
	return true
}

func (m *Model) clearSuggestionSelection() bool {
	if m.suggestionIndex == -1 {
		return false
	}
	m.prompt.SetValue(m.suggestionOriginal)
	m.prompt.CursorEnd()
	m.suggestionIndex = -1
	m.updateSuggestionWindow()
	m.refreshSuggestionOverlay()
	return true
}
