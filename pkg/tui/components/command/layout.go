package command

// SetSize configures the viewport dimensions the component manages.
func (m *Model) SetSize(width, height int) {
	if width <= 0 {
		width = 1
	}
	if height <= 1 {
		height = 2
	}
	m.width = width
	m.height = height
	m.contentHeight = height - 1
	if m.contentHeight <= 0 {
		m.contentHeight = 1
	}
	promptWidth := width - len(m.promptPrefix)
	if promptWidth < 5 {
		promptWidth = width - 1
		if promptWidth < 1 {
			promptWidth = 1
		}
	}
	m.prompt.SetWidth(promptWidth)
	m.updateSuggestionWindow()
	m.refreshSuggestionOverlay()
}
