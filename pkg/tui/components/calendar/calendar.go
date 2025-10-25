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

// Render produces a multi-line calendar string for the given month.
func Render(month time.Time, days []Day, opts Options) string {
	if month.IsZero() {
		return ""
	}

	first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
	daysInMonth := daysIn(month)

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

func daysIn(month time.Time) int {
	first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
	return first.AddDate(0, 1, -1).Day()
}
