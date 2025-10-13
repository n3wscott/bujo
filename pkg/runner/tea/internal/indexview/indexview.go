package indexview

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/runner/tea/internal/calendar"
)

// State tracks collection-fold state and calendar metadata for the index pane.
type State struct {
	Fold           map[string]bool
	Selection      map[string]int
	Months         map[string]*MonthState
	ActiveMonthKey string
}

// NewState creates a fresh State with initialized maps.
func NewState() *State {
	return &State{
		Fold:      make(map[string]bool),
		Selection: make(map[string]int),
		Months:    make(map[string]*MonthState),
	}
}

func (s *State) ensure() {
	if s.Fold == nil {
		s.Fold = make(map[string]bool)
	}
	if s.Selection == nil {
		s.Selection = make(map[string]int)
	}
	if s.Months == nil {
		s.Months = make(map[string]*MonthState)
	}
}

// CollectionItem represents a collection row in the index list.
type CollectionItem struct {
	Name        string
	Resolved    string
	Active      bool
	Indent      bool
	Folded      bool
	HasChildren bool
}

// Title renders the item title for the list delegate.
func (c CollectionItem) Title() string {
	label := c.displayLabel()
	if c.Active {
		return "→ " + label
	}
	return "  " + label
}

// Description returns the entry description (unused).
func (c CollectionItem) Description() string { return "" }

// FilterValue returns text used by list filtering.
func (c CollectionItem) FilterValue() string {
	if c.Resolved == "" || c.Resolved == c.Name {
		return c.Name
	}
	return c.Name + " " + c.Resolved
}

func (c CollectionItem) displayLabel() string {
	label := c.Name
	if c.Indent {
		if t := parseFriendlyDate(label); !t.IsZero() {
			label = fmt.Sprintf("%s - %s", t.Format("2"), t.Format("Monday"))
		}
		label = "  " + label
	} else if c.HasChildren {
		if c.Folded {
			label = "▸ " + label
		} else {
			label = "▾ " + label
		}
	}
	return label
}

// CalendarHeaderItem is the weekday header row for a calendar month.
type CalendarHeaderItem struct {
	Month string
	Text  string
}

// Title renders the calendar header label.
func (ci *CalendarHeaderItem) Title() string { return ci.Text }

// Description returns the header description (unused).
func (ci *CalendarHeaderItem) Description() string { return "" }

// FilterValue exposes the month for filtering.
func (ci *CalendarHeaderItem) FilterValue() string { return ci.Month }

// CalendarRowItem is a row of days in a rendered calendar month.
type CalendarRowItem struct {
	Month    string
	Week     int
	Days     []int
	Text     string
	RowIndex int
}

// Title renders the calendar week row text.
func (ci *CalendarRowItem) Title() string { return ci.Text }

// Description returns the row description (unused).
func (ci *CalendarRowItem) Description() string { return "" }

// FilterValue exposes the row's parent month for filtering.
func (ci *CalendarRowItem) FilterValue() string { return ci.Month }

// MonthState tracks calendar rendering data for a month entry.
type MonthState struct {
	Month     string
	MonthTime time.Time
	Children  []CollectionItem
	HeaderIdx int
	Weeks     []*CalendarRowItem
}

// BuildItems constructs list items for the index pane, updating state in place.
func BuildItems(state *State, cols []string, currentResolved string, now time.Time) []list.Item {
	state.ensure()
	for key := range state.Months {
		delete(state.Months, key)
	}

	todayMonth := now.Format("January 2006")

	type monthEntry struct {
		name string
		time time.Time
		base CollectionItem
	}

	monthChildren := make(map[string][]CollectionItem)
	monthEntries := make(map[string]*monthEntry)
	otherChildren := make(map[string][]CollectionItem)
	otherBases := make(map[string]CollectionItem)
	otherOrder := make([]string, 0)

	addOtherBase := func(name string, item CollectionItem) {
		if _, ok := otherBases[name]; !ok {
			otherBases[name] = item
			otherOrder = append(otherOrder, name)
		}
	}

	for _, raw := range cols {
		parts := strings.SplitN(raw, "/", 2)
		if len(parts) == 2 {
			parent, child := parts[0], parts[1]
			if t, isMonth := ParseMonth(parent); isMonth {
				monthChildren[parent] = append(monthChildren[parent], CollectionItem{Name: child, Resolved: raw, Indent: true})
				if _, ok := monthEntries[parent]; !ok {
					monthEntries[parent] = &monthEntry{name: parent, time: t}
				}
			} else {
				otherChildren[parent] = append(otherChildren[parent], CollectionItem{Name: child, Resolved: raw, Indent: true})
				addOtherBase(parent, CollectionItem{Name: parent, Resolved: parent})
			}
			continue
		}

		if t, isMonth := ParseMonth(raw); isMonth {
			entry := monthEntries[raw]
			if entry == nil {
				entry = &monthEntry{name: raw, time: t}
				monthEntries[raw] = entry
			} else if entry.time.IsZero() {
				entry.time = t
			}
			entry.base = CollectionItem{Name: raw, Resolved: raw}
			continue
		}

		addOtherBase(raw, CollectionItem{Name: raw, Resolved: raw})
	}

	if _, ok := monthEntries[todayMonth]; !ok {
		if t, isMonth := ParseMonth(todayMonth); isMonth {
			monthEntries[todayMonth] = &monthEntry{
				name: todayMonth,
				time: t,
				base: CollectionItem{Name: todayMonth, Resolved: todayMonth},
			}
		}
	}

	months := make([]*monthEntry, 0, len(monthEntries))
	for name, entry := range monthEntries {
		if entry.base.Name == "" {
			entry.base = CollectionItem{Name: name, Resolved: name}
		}
		if entry.time.IsZero() {
			if t, ok := ParseMonth(name); ok {
				entry.time = t
			}
		}
		months = append(months, entry)
	}
	sort.Slice(months, func(i, j int) bool {
		ti, tj := months[i].time, months[j].time
		switch {
		case ti.Equal(tj):
			return months[i].name > months[j].name
		case ti.IsZero():
			return false
		case tj.IsZero():
			return true
		default:
			return ti.After(tj)
		}
	})

	result := make([]list.Item, 0, len(cols)+16)

	appendCollection := func(base CollectionItem, children []CollectionItem, monthTime time.Time, isMonth bool) {
		key := base.Resolved
		if key == "" {
			key = base.Name
		}
		if isMonth {
			if _, ok := state.Fold[key]; !ok {
				state.Fold[key] = base.Name != todayMonth && base.Resolved != todayMonth
			}
		}
		base.HasChildren = isMonth || len(children) > 0
		base.Folded = state.Fold[key]
		result = append(result, base)

		if !isMonth {
			if len(children) > 0 {
				sortCollectionChildren(children)
				if !base.Folded {
					for _, child := range children {
						result = append(result, child)
					}
				}
			}
			return
		}

		state.Months[base.Resolved] = &MonthState{
			Month:     base.Resolved,
			MonthTime: monthTime,
			Children:  append([]CollectionItem(nil), children...),
			HeaderIdx: -1,
		}

		selected := state.Selection[base.Resolved]
		if selected == 0 {
			selected = DefaultSelectedDay(base.Resolved, monthTime, children, currentResolved, now)
			if selected > 0 {
				state.Selection[base.Resolved] = selected
			}
		}

		if base.Folded {
			return
		}

		selectedForRender := selected
		if state.ActiveMonthKey != base.Resolved {
			selectedForRender = 0
		}

		header, weeks := RenderCalendarRows(base.Resolved, monthTime, children, selectedForRender, now, DefaultCalendarOptions())
		if header == nil {
			return
		}
		state.Months[base.Resolved].HeaderIdx = len(result)
		result = append(result, header)
		for _, week := range weeks {
			week.RowIndex = len(result)
			result = append(result, week)
		}
		state.Months[base.Resolved].Weeks = weeks
	}

	for _, entry := range months {
		appendCollection(entry.base, monthChildren[entry.name], entry.time, true)
	}

	for _, name := range otherOrder {
		base := otherBases[name]
		children := otherChildren[name]
		appendCollection(base, children, time.Time{}, false)
	}

	return result
}

// RenderCalendarRows produces header and week rows for a month.
func RenderCalendarRows(month string, monthTime time.Time, children []CollectionItem, selectedDay int, now time.Time, opts calendar.Options) (*CalendarHeaderItem, []*CalendarRowItem) {
	header := &CalendarHeaderItem{
		Month: month,
		Text:  "  " + opts.HeaderStyle.Render("Su Mo Tu We Th Fr Sa"),
	}

	entryDays := make(map[int]bool)
	for _, child := range children {
		if day := DayNumberFromName(monthTime, child.Name); day > 0 {
			entryDays[day] = true
		}
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

	weeks := make([]*CalendarRowItem, 0, rowsCount)
	for row := 0; row < rowsCount; row++ {
		var cells []string
		weekDays := make([]int, 7)
		for col := 0; col < 7; col++ {
			cellIdx := row*7 + col
			day := cellIdx - offset + 1
			if day < 1 || day > daysInMonth {
				cells = append(cells, opts.EmptyStyle.Render("  "))
				weekDays[col] = 0
				continue
			}
			style := opts.EmptyStyle
			if entryDays[day] {
				style = opts.EntryStyle
			}
			if day == todayDay {
				style = style.Inherit(opts.TodayStyle)
			}
			if selectedDay > 0 && day == selectedDay {
				style = style.Inherit(opts.SelectedStyle)
			}
			cells = append(cells, style.Render(fmt.Sprintf("%2d", day)))
			weekDays[col] = day
		}
		text := "  " + strings.Join(cells, " ")
		weekItem := &CalendarRowItem{
			Month: month,
			Week:  row,
			Days:  weekDays,
			Text:  text,
		}
		weeks = append(weeks, weekItem)
	}

	return header, weeks
}

func sortCollectionChildren(children []CollectionItem) {
	sort.SliceStable(children, func(i, j int) bool {
		ti := parseFriendlyDate(children[i].Name)
		tj := parseFriendlyDate(children[j].Name)
		if !ti.IsZero() && !tj.IsZero() {
			return ti.Before(tj)
		}
		if ti.IsZero() != tj.IsZero() {
			return !ti.IsZero()
		}
		return strings.Compare(children[i].Name, children[j].Name) < 0
	})
}

func parseFriendlyDate(s string) time.Time {
	layouts := []string{"January 2, 2006", "2006-01-02"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// DefaultCalendarOptions returns styling used for calendar rendering.
func DefaultCalendarOptions() calendar.Options {
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Bold(true)
	empty := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	entry := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	today := lipgloss.NewStyle().Underline(true)
	selected := lipgloss.NewStyle().Background(lipgloss.Color("63")).Foreground(lipgloss.Color("0"))
	return calendar.Options{
		HeaderStyle:   header,
		EmptyStyle:    empty,
		EntryStyle:    entry,
		TodayStyle:    today,
		SelectedStyle: selected,
		ShowHeader:    true,
	}
}

// DefaultSelectedDay chooses the initial day to highlight for a month.
func DefaultSelectedDay(month string, monthTime time.Time, children []CollectionItem, currentResolved string, now time.Time) int {
	if strings.HasPrefix(currentResolved, month+"/") {
		if day := DayFromPath(currentResolved); day > 0 {
			return day
		}
	}
	if monthTime.Year() == now.Year() && monthTime.Month() == now.Month() {
		return now.Day()
	}
	for _, child := range children {
		if day := DayNumberFromName(monthTime, child.Name); day > 0 {
			return day
		}
	}
	return 0
}

// ParseMonth attempts to parse a collection name as "January 2006".
func ParseMonth(name string) (time.Time, bool) {
	if t, err := time.Parse("January 2006", name); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// DayNumberFromName parses a child collection label into a day number.
func DayNumberFromName(monthTime time.Time, name string) int {
	t, err := time.Parse("January 2, 2006", name)
	if err != nil {
		return 0
	}
	if t.Year() != monthTime.Year() || t.Month() != monthTime.Month() {
		return 0
	}
	return t.Day()
}

// DayFromPath extracts the day number from a resolved "Month/Day" path.
func DayFromPath(path string) int {
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return 0
	}
	t, err := time.Parse("January 2, 2006", parts[1])
	if err != nil {
		return 0
	}
	return t.Day()
}

// FormatDayPath formats the calendar selection into "Month/Day".
func FormatDayPath(monthTime time.Time, day int) string {
	dt := time.Date(monthTime.Year(), monthTime.Month(), day, 0, 0, 0, 0, monthTime.Location())
	return fmt.Sprintf("%s/%s", monthTime.Format("January 2006"), dt.Format("January 2, 2006"))
}

// FirstNonZero returns the first non-zero day in a slice.
func FirstNonZero(days []int) int {
	for _, d := range days {
		if d > 0 {
			return d
		}
	}
	return 0
}

// ContainsDay reports whether the target day exists in the slice.
func ContainsDay(days []int, target int) bool {
	for _, d := range days {
		if d == target {
			return true
		}
	}
	return false
}

// DaysIn returns the number of days in the month represented by monthTime.
func DaysIn(month time.Time) int {
	first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
	return first.AddDate(0, 1, -1).Day()
}
