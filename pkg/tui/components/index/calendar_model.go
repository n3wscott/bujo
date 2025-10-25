package index

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
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

// New creates a calendar model for the provided month.
func NewCalendarModel(month string, selected int, now time.Time) *CalendarModel {
	m := &CalendarModel{
		monthName: month,
		current:   now,
		selected:  selected,
		children:  nil,
	}
	m.recompute()
	return m
}

// Init implements tea.Model.
func (m *CalendarModel) Init() tea.Cmd { return nil }

// Update handles navigation keys (hjkl/arrow keys) and window sizing.
func (m *CalendarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		m.handleMovement(msg.String())
	}
	return m, nil
}

// SetMonth updates the rendered month.
func (m *CalendarModel) SetMonth(month string) {
	m.monthName = month
	m.recompute()
}

// SetChildren provides child collection items (used to mark days with entries).
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

// View renders the current calendar string.
func (m *CalendarModel) View() string {
	if m.header == nil {
		return ""
	}
	lines := []string{m.header.Text}
	for _, row := range m.rows {
		lines = append(lines, row.Text)
	}
	return lipgloss.NewStyle().Render(strings.Join(lines, "\n"))
}

// Header returns the rendered header item.
func (m *CalendarModel) Header() *CalendarHeaderItem {
	return m.header
}

// Rows returns the rendered week rows.
func (m *CalendarModel) Rows() []*CalendarRowItem {
	return m.rows
}

// Selected returns the highlighted day number.
func (m *CalendarModel) Selected() int {
	return m.selected
}

func (m *CalendarModel) moveSelection(delta int) {
	if delta == 0 {
		return
	}
	monthTime, ok := ParseMonth(m.monthName)
	if !ok {
		return
	}
	days := DaysIn(monthTime)
	if days == 0 {
		return
	}
	next := m.selected + delta
	if next < 1 {
		next = 1
	}
	if next > days {
		next = days
	}
	m.selected = next
	m.recompute()
}

func (m *CalendarModel) handleMovement(key string) {
	switch key {
	case "left", "h":
		m.moveSelection(-1)
	case "right", "l":
		m.moveSelection(1)
	case "up", "k":
		m.moveSelection(-7)
	case "down", "j":
		m.moveSelection(7)
	}
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
	m.header = header
	m.rows = rows
}
