package index

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
)

// CalendarModel renders a month calendar with selection handling.
type CalendarModel struct {
	monthName string
	current   time.Time
	selected  int

	children []CollectionItem

	header *CalendarHeaderItem
	rows   []*CalendarRowItem

	width  int
	height int
}

// NewCalendarModel creates a calendar model for the provided month.
func NewCalendarModel(month string, selected int, now time.Time) *CalendarModel {
	m := &CalendarModel{
		monthName: month,
		current:   now,
		selected:  selected,
	}
	m.recompute()
	return m
}

// Init implements tea.Model.
func (m *CalendarModel) Init() tea.Cmd { return nil }

// Update handles navigation keys (hjkl/arrow keys) and window sizing.
func (m *CalendarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		cmd = m.handleMovement(msg.String())
	}
	return m, cmd
}

// SetMonth updates the rendered month.
func (m *CalendarModel) SetMonth(month string) {
	m.monthName = month
	m.recompute()
}

// SetChildren marks which days have entries based on collection children.
func (m *CalendarModel) SetChildren(children []CollectionItem) {
	m.children = append([]CollectionItem(nil), children...)
	m.recompute()
}

// SetNow updates the reference time (for marking today).
func (m *CalendarModel) SetNow(now time.Time) {
	m.current = now
	m.recompute()
}

// SetSelected sets the highlighted day (1-indexed).
func (m *CalendarModel) SetSelected(day int) {
	m.selected = day
	m.recompute()
}

// Header returns the rendered header item.
func (m *CalendarModel) Header() *CalendarHeaderItem {
	return m.header
}

// Rows returns the rendered week rows.
func (m *CalendarModel) Rows() []*CalendarRowItem {
	return m.rows
}

// View renders the current calendar string.
func (m *CalendarModel) View() string {
	if m.header == nil {
		return ""
	}
	lines := []string{m.header.Text}
	for _, row := range m.rows {
		lines = append(lines, row.Text)
	}
	return strings.Join(lines, "\n")
}

func (m *CalendarModel) handleMovement(key string) tea.Cmd {
	switch key {
	case "left", "h":
		return m.moveSelection(-1)
	case "right", "l":
		return m.moveSelection(1)
	case "up", "k":
		return m.moveSelection(-7)
	case "down", "j":
		return m.moveSelection(7)
	}
	return nil
}

func (m *CalendarModel) moveSelection(delta int) tea.Cmd {
	if delta == 0 {
		return nil
	}
	monthTime, ok := ParseMonth(m.monthName)
	if !ok {
		return nil
	}
	days := DaysIn(monthTime)
	if days == 0 {
		return nil
	}
	next := m.selected + delta
	if next < 1 {
		return focusCmd(m.monthName, -1)
	}
	if next > days {
		return focusCmd(m.monthName, 1)
	}
	m.selected = next
	m.recompute()
	return nil
}

func (m *CalendarModel) recompute() {
	monthTime, ok := ParseMonth(m.monthName)
	if !ok {
		m.header = nil
		m.rows = nil
		return
	}
	header, rows := RenderCalendarRows(
		m.monthName,
		monthTime,
		m.children,
		m.selected,
		m.current,
		DefaultCalendarOptions(),
	)
	if header == nil {
		m.header = nil
		m.rows = nil
		return
	}

	m.header = header
	m.rows = rows
}

// CalendarFocusMsg indicates navigation moved beyond the calendar bounds.
type CalendarFocusMsg struct {
	Month     string
	Direction int // -1 up/out, +1 down/out
}

func focusCmd(month string, dir int) tea.Cmd {
	return func() tea.Msg {
		return CalendarFocusMsg{Month: month, Direction: dir}
	}
}
