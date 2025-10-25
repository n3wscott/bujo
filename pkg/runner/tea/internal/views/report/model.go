package report

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/runner/tea/internal/theme"
	"tableflip.dev/bujo/pkg/runner/tea/internal/uiutil"
)

// Model renders and scrolls the completion report overlay.
type Model struct {
	sections []app.ReportSection
	lines    []string
	offset   int

	label string
	since time.Time
	until time.Time
	total int

	viewportWidth  int
	viewportHeight int

	theme        theme.Theme
	relativeTime func(time.Time, time.Time) string
}

// New creates a report overlay model.
func New(th theme.Theme, relative func(time.Time, time.Time) string) *Model {
	if relative == nil {
		relative = func(time.Time, time.Time) string { return "" }
	}
	return &Model{
		theme:        th,
		relativeTime: relative,
	}
}

// Active reports whether the overlay has content.
func (m *Model) Active() bool {
	return len(m.lines) > 0
}

// Clear removes current report data.
func (m *Model) Clear() {
	m.sections = nil
	m.lines = nil
	m.offset = 0
	m.label = ""
	m.since = time.Time{}
	m.until = time.Time{}
	m.total = 0
}

// SetViewport configures the usable width/height for rendering.
func (m *Model) SetViewport(totalWidth, availableHeight int) {
	width := totalWidth - 6
	if width < 20 {
		width = 20
	}
	if availableHeight < 3 {
		availableHeight = 3
	}
	m.viewportWidth = width
	m.viewportHeight = availableHeight
	m.ensureBounds()
}

// SetData stores the report result and rebuilds the rendered lines.
func (m *Model) SetData(label string, since, until time.Time, total int, sections []app.ReportSection) {
	m.sections = append([]app.ReportSection(nil), sections...)
	m.label = label
	m.since = since
	m.until = until
	m.total = total
	m.lines = m.buildLines(time.Now())
	m.offset = 0
	m.ensureBounds()
}

// ScrollLines moves the viewport by delta lines.
func (m *Model) ScrollLines(delta int) {
	if delta == 0 {
		return
	}
	m.offset += delta
	m.ensureBounds()
}

// ScrollPages moves the viewport by page deltas.
func (m *Model) ScrollPages(delta int) {
	if delta == 0 {
		return
	}
	m.offset += delta * m.viewportHeight
	m.ensureBounds()
}

// ScrollHome jumps to the start of the overlay.
func (m *Model) ScrollHome() {
	m.offset = 0
	m.ensureBounds()
}

// ScrollEnd jumps to the end of the overlay.
func (m *Model) ScrollEnd() {
	m.offset = len(m.lines)
	m.ensureBounds()
}

// View returns the rendered report overlay.
func (m *Model) View() string {
	if len(m.lines) == 0 || m.viewportHeight == 0 {
		return ""
	}
	m.ensureBounds()
	height := m.viewportHeight
	if height > len(m.lines) {
		height = len(m.lines)
	}
	end := m.offset + height
	if end > len(m.lines) {
		end = len(m.lines)
	}
	viewport := m.lines[m.offset:end]
	width := m.viewportWidth
	padded := make([]string, len(viewport))
	for i, line := range viewport {
		padded[i] = padRight(line, width)
	}
	frame := m.theme.Report.Frame.Copy().Width(width + 4)
	return frame.Render(strings.Join(padded, "\n"))
}

func (m *Model) buildLines(now time.Time) []string {
	header := m.theme.Report.Header.Render(
		fmt.Sprintf("Report · last %s (%s → %s)", m.label, uiutil.FormatReportTime(m.since), uiutil.FormatReportTime(m.until)),
	)
	summary := m.theme.Report.Text.Render(fmt.Sprintf("%d completed entries", m.total))
	lines := []string{header, summary, ""}

	if m.total == 0 {
		return append(lines, m.theme.Report.Text.Render("No completed entries found in this window."))
	}

	for _, sec := range m.sections {
		name := uiutil.FormattedCollectionName(sec.Collection)
		if strings.TrimSpace(name) == "" {
			name = sec.Collection
		}
		lines = append(lines, m.theme.Report.Header.Render(name))
		included := make(map[string]*entry.Entry, len(sec.Entries))
		for i := range sec.Entries {
			if sec.Entries[i].Entry != nil {
				included[sec.Entries[i].Entry.ID] = sec.Entries[i].Entry
			}
		}
		for _, item := range sec.Entries {
			ent := item.Entry
			if ent == nil {
				continue
			}
			depth := reportDepth(ent, included)
			indent := strings.Repeat("  ", depth)
			signifier := ent.Signifier.String()
			if signifier == "" {
				signifier = " "
			}
			bullet := ent.Bullet.Glyph().Symbol
			if bullet == "" {
				bullet = ent.Bullet.String()
			}
			message := ent.Message
			if strings.TrimSpace(message) == "" {
				message = "<empty>"
			}
			line := fmt.Sprintf("  %s%s %s %s", indent, signifier, bullet, message)
			if item.Completed {
				line = fmt.Sprintf("%s  · %s", line, m.relativeTime(item.CompletedAt, now))
			}
			lines = append(lines, m.theme.Report.Text.Render(line))
		}
		lines = append(lines, "")
	}
	return lines
}

func (m *Model) ensureBounds() {
	if len(m.lines) == 0 {
		m.offset = 0
		return
	}
	height := m.viewportHeight
	if height <= 0 {
		height = 1
	}
	maxOffset := len(m.lines) - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func reportDepth(e *entry.Entry, included map[string]*entry.Entry) int {
	if e == nil {
		return 0
	}
	depth := 0
	visited := make(map[string]bool)
	parentID := e.ParentID
	for parentID != "" {
		if visited[parentID] {
			break
		}
		visited[parentID] = true
		parent, ok := included[parentID]
		if !ok {
			break
		}
		depth++
		parentID = parent.ParentID
	}
	return depth
}
