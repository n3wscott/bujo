package addtask

// SetSize configures the overlay dimensions.
func (m *Model) SetSize(width, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	m.width = width
	m.height = height
	usable := width - 8
	if usable < 14 {
		usable = width - 6
	}
	if usable < 12 {
		usable = 12
	}
	m.fieldWidth = usable
	inputWidth := usable - 16
	if inputWidth < 12 {
		inputWidth = max(10, usable-4)
	}
	m.taskInput.SetWidth(inputWidth)
}

func clampInt(value, minVal, maxVal int) int {
	if maxVal > 0 && value > maxVal {
		value = maxVal
	}
	if value < minVal {
		value = minVal
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
