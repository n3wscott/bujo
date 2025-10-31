package overlay

import (
	"strings"

	"github.com/charmbracelet/lipgloss/v2"
)

// Placement controls overlay alignment and sizing.
type Placement struct {
	Horizontal lipgloss.Position
	Vertical   lipgloss.Position
	MarginX    int
	MarginY    int
	Width      int
	Height     int
}

// Compose overlays the foreground view atop the background while preserving
// background content outside the overlay bounds.
func Compose(background string, width, height int, foreground string, placement Placement) string {
	bgLines := normalizeBackground(background, width, height)
	if foreground == "" {
		return strings.Join(bgLines, "\n")
	}

	fgLines := strings.Split(foreground, "\n")
	if len(fgLines) == 0 {
		return strings.Join(bgLines, "\n")
	}

	overlayWidth := placement.Width
	if overlayWidth <= 0 {
		for _, line := range fgLines {
			if w := lipgloss.Width(line); w > overlayWidth {
				overlayWidth = w
			}
		}
	}
	if overlayWidth <= 0 {
		return strings.Join(bgLines, "\n")
	}
	if overlayWidth > width {
		overlayWidth = width
	}

	overlayHeight := placement.Height
	if overlayHeight <= 0 {
		overlayHeight = len(fgLines)
	}
	if overlayHeight <= 0 {
		return strings.Join(bgLines, "\n")
	}
	if overlayHeight > height {
		overlayHeight = height
	}

	offsetX, offsetY := computeOffsets(width, height, overlayWidth, overlayHeight, placement)

	for row := 0; row < overlayHeight; row++ {
		destY := offsetY + row
		if destY < 0 || destY >= len(bgLines) {
			continue
		}
		fgLine := ""
		if row < len(fgLines) {
			fgLine = fgLines[row]
		}
		fgLine = padToWidth(fgLine, overlayWidth)

		baseLine := bgLines[destY]
		prefix := sliceWidth(baseLine, 0, offsetX)
		suffix := sliceWidth(baseLine, offsetX+overlayWidth, width)
		bgLines[destY] = prefix + fgLine + suffix
	}

	return strings.Join(bgLines, "\n")
}

func normalizeBackground(view string, width, height int) []string {
	lines := strings.Split(view, "\n")
	if len(lines) > height {
		lines = lines[len(lines)-height:]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i := range lines {
		lines[i] = padToWidth(lines[i], width)
	}
	return lines
}

func padToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	currWidth := lipgloss.Width(s)
	if currWidth >= width {
		return lipgloss.NewStyle().Width(width).Render(s)
	}
	return s + strings.Repeat(" ", width-currWidth)
}

func sliceWidth(s string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > lipgloss.Width(s) {
		end = lipgloss.Width(s)
	}
	if start >= end {
		return ""
	}

	runes := []rune(s)
	result := strings.Builder{}
	widthSeen := 0
	for _, r := range runes {
		rw := lipgloss.Width(string(r))
		next := widthSeen + rw
		if next <= start {
			widthSeen = next
			continue
		}
		if widthSeen >= end {
			break
		}
		if next > end {
			break
		}
		result.WriteRune(r)
		widthSeen = next
	}
	return result.String()
}

func computeOffsets(width, height, overlayWidth, overlayHeight int, placement Placement) (int, int) {
	h := placement.Horizontal
	if h == 0 {
		h = lipgloss.Center
	}
	v := placement.Vertical
	if v == 0 {
		v = lipgloss.Center
	}

	offsetX := placement.MarginX
	switch h {
	case lipgloss.Right:
		offsetX = width - overlayWidth - placement.MarginX
	case lipgloss.Center:
		offsetX = (width - overlayWidth) / 2
	}
	if offsetX < 0 {
		offsetX = 0
	}
	if offsetX > width-overlayWidth {
		offsetX = width - overlayWidth
	}

	offsetY := placement.MarginY
	switch v {
	case lipgloss.Bottom:
		offsetY = height - overlayHeight - placement.MarginY
	case lipgloss.Center:
		offsetY = (height - overlayHeight) / 2
	}
	if offsetY < 0 {
		offsetY = 0
	}
	if offsetY > height-overlayHeight {
		offsetY = height - overlayHeight
	}

	return offsetX, offsetY
}
