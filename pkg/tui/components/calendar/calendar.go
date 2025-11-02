// Package calendar provides helpers for rendering calendar views.
package calendar

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/v2"
)

// Day describes a single day rendered in the calendar.
type Day struct {
	Day        int
	HasEntry   bool
	IsToday    bool
	IsSelected bool
}

// Options controls calendar styling.
type Options struct {
	HeaderStyle   lipgloss.Style
	EmptyStyle    lipgloss.Style
	EntryStyle    lipgloss.Style
	TodayStyle    lipgloss.Style
	SelectedStyle lipgloss.Style
	ShowHeader    bool
}

// HeaderItem represents the rendered weekday header.
type HeaderItem struct {
	Month string
	Text  string
}

// Title renders the header label.
func (ci *HeaderItem) Title() string { return ci.Text }

// Description returns the header description (unused).
func (ci *HeaderItem) Description() string { return "" }

// FilterValue exposes the month for filtering.
func (ci *HeaderItem) FilterValue() string { return ci.Month }

// RowItem represents a calendar week row.
type RowItem struct {
	Month    string
	Week     int
	Days     []int
	Text     string
	RowIndex int
}

// Title renders the row label.
func (ci *RowItem) Title() string { return ci.Text }

// Description returns the row description (unused).
func (ci *RowItem) Description() string { return "" }

// FilterValue exposes the month.
func (ci *RowItem) FilterValue() string { return ci.Month }

// Render produces a multi-line calendar string for the given month.
func Render(month time.Time, days []Day, opts Options) string {
	if month.IsZero() {
		return ""
	}

	first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
	daysInMonth := DaysIn(month)

	byDay := make(map[int]Day, len(days))
	for _, d := range days {
		if d.Day >= 1 && d.Day <= daysInMonth {
			byDay[d.Day] = d
		}
	}

	var lines []string
	if opts.ShowHeader {
		lines = append(lines, opts.HeaderStyle.Render("Su Mo Tu We Th Fr Sa"))
	}

	startOffset := int(first.Weekday())
	totalCells := startOffset + daysInMonth
	rows := (totalCells + 6) / 7

	for row := 0; row < rows; row++ {
		var cells []string
		for col := 0; col < 7; col++ {
			cellIdx := row*7 + col
			day := cellIdx - startOffset + 1
			if day < 1 || day > daysInMonth {
				cells = append(cells, opts.EmptyStyle.Render("  "))
				continue
			}
			cells = append(cells, renderDay(byDay[day], day, opts))
		}
		lines = append(lines, strings.Join(cells, " "))
	}

	return strings.Join(lines, "\n")
}

func renderDay(info Day, day int, opts Options) string {
	text := fmt.Sprintf("%2d", day)

	style := opts.EmptyStyle
	if info.HasEntry {
		style = opts.EntryStyle
	}
	if info.IsToday {
		style = style.Inherit(opts.TodayStyle)
	}
	if info.IsSelected {
		style = style.Inherit(opts.SelectedStyle)
	}
	return style.Render(text)
}

// DaysIn returns the number of days in a month.
func DaysIn(month time.Time) int {
	first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
	return first.AddDate(0, 1, -1).Day()
}

// ParseMonth attempts to parse "January 2006" names.
func ParseMonth(name string) (time.Time, bool) {
	if strings.TrimSpace(name) == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("January 2006", name)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// DefaultOptions returns the styling used for calendar rendering.
func DefaultOptions() Options {
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Bold(true)
	empty := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	entry := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	today := lipgloss.NewStyle().Underline(true)
	selected := lipgloss.NewStyle().Background(lipgloss.Color("63")).Foreground(lipgloss.Color("0"))
	return Options{
		HeaderStyle:   header,
		EmptyStyle:    empty,
		EntryStyle:    entry,
		TodayStyle:    today,
		SelectedStyle: selected,
		ShowHeader:    true,
	}
}

// RenderRows produces header and week rows for a month.
func RenderRows(month string, monthTime time.Time, entryDays map[int]bool, selectedDay int, now time.Time, opts Options) (*HeaderItem, []*RowItem) {
	header := &HeaderItem{
		Month: month,
		Text:  "  " + opts.HeaderStyle.Render("Su Mo Tu We Th Fr Sa"),
	}

	if entryDays == nil {
		entryDays = make(map[int]bool)
	}

	todayDay := 0
	if monthTime.Year() == now.Year() && monthTime.Month() == now.Month() {
		todayDay = now.Day()
	}

	first := time.Date(monthTime.Year(), monthTime.Month(), 1, 0, 0, 0, 0, monthTime.Location())
	offset := int(first.Weekday())
	daysInMonth := DaysIn(monthTime)
	totalCells := offset + daysInMonth
	rowsCount := (totalCells + 6) / 7

	rows := make([]*RowItem, 0, rowsCount)
	for row := 0; row < rowsCount; row++ {
		var cells []string
		days := make([]int, 0, 7)
		for col := 0; col < 7; col++ {
			cellIdx := row*7 + col
			day := cellIdx - offset + 1
			if day < 1 || day > daysInMonth {
				cells = append(cells, opts.EmptyStyle.Render("  "))
				days = append(days, 0)
				continue
			}
			info := Day{
				Day:        day,
				HasEntry:   entryDays[day],
				IsToday:    day == todayDay,
				IsSelected: day == selectedDay && selectedDay > 0,
			}
			cells = append(cells, renderDay(info, day, opts))
			days = append(days, day)
		}
		rows = append(rows, &RowItem{
			Month: month,
			Week:  row,
			Days:  days,
			Text:  "  " + strings.Join(cells, " "),
		})
	}
	return header, rows
}
