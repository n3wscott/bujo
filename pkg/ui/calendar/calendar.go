package calendar

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/v2"
)

// Day describes metadata used when rendering the calendar.
type Day struct {
	Day        int
	HasEntry   bool
	IsToday    bool
	IsSelected bool
}

// Options controls the styling of the rendered calendar.
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

	firstOfMonth := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
	daysInMonth := daysIn(month)

	meta := make(map[int]Day, len(days))
	for _, d := range days {
		if d.Day >= 1 && d.Day <= daysInMonth {
			meta[d.Day] = d
		}
	}

	var lines []string
	if opts.ShowHeader {
		header := opts.HeaderStyle.Render("Su Mo Tu We Th Fr Sa")
		lines = append(lines, header)
	}

	weekdayOffset := int(firstOfMonth.Weekday()) // Sunday == 0
	rows := ((weekdayOffset + daysInMonth) + 6) / 7
	for row := 0; row < rows; row++ {
		var cells []string
		for col := 0; col < 7; col++ {
			cellIndex := row*7 + col
			day := cellIndex - weekdayOffset + 1
			if day < 1 || day > daysInMonth {
				cells = append(cells, opts.EmptyStyle.Render("  "))
				continue
			}
			cells = append(cells, renderDay(meta[day], day, opts))
		}
		lines = append(lines, strings.TrimRight(strings.Join(cells, " "), " "))
	}

	return strings.Join(lines, "\n")
}

func renderDay(info Day, day int, opts Options) string {
	glyph := dayGlyph(day)

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
	return style.Render(glyph)
}

func daysIn(month time.Time) int {
	first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
	return first.AddDate(0, 1, -1).Day()
}

func dayGlyph(day int) string {
	if day < 0 || day >= len(whiteCircledDigits) {
		return "  "
	}
	return whiteCircledDigits[day]
}

var whiteCircledDigits = []string{
	"⓪",
	"①", "②", "③", "④", "⑤", "⑥", "⑦", "⑧", "⑨", "⑩",
	"⑪", "⑫", "⑬", "⑭", "⑮", "⑯", "⑰", "⑱", "⑲", "⑳",
	"㉑", "㉒", "㉓", "㉔", "㉕", "㉖", "㉗", "㉘", "㉙", "㉚",
	"㉛", "㉜", "㉝", "㉞", "㉟",
	"㊱", "㊲", "㊳", "㊴", "㊵", "㊶", "㊷", "㊸", "㊹", "㊺", "㊻", "㊼", "㊽", "㊾", "㊿",
}
