package collectionnav

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/tui/components/index"
)

func (m *Model) pruneCalendars() {
	if len(m.calendars) == 0 {
		return
	}
	valid := make(map[string]struct{})
	var stack []*viewmodel.ParsedCollection
	stack = append(stack, m.roots...)
	for len(stack) > 0 {
		last := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if last == nil {
			continue
		}
		valid[last.ID] = struct{}{}
		if len(last.Children) > 0 {
			stack = append(stack, last.Children...)
		}
	}
	for id := range m.calendars {
		if _, ok := valid[id]; !ok {
			delete(m.calendars, id)
		}
	}
	if len(m.calendarExtras) > 0 {
		for id := range m.calendarExtras {
			if _, ok := valid[id]; !ok {
				delete(m.calendarExtras, id)
			}
		}
	}
}

func (m *Model) ensureCalendar(col *viewmodel.ParsedCollection) *index.CalendarModel {
	if col == nil {
		return nil
	}
	cal, ok := m.calendars[col.ID]
	if !ok {
		cal = index.NewCalendarModel(col.Name, 0, m.now())
		m.calendars[col.ID] = cal
	}
	cal.SetNow(m.now())
	cal.SetMonth(col.Name)
	cal.SetChildren(m.calendarChildren(col))
	return cal
}

func (m *Model) calendarChildren(col *viewmodel.ParsedCollection) []index.CollectionItem {
	if col == nil {
		return nil
	}
	items := make(map[int]index.CollectionItem)
	addItem := func(name, resolved string, date time.Time) {
		if resolved == "" {
			return
		}
		dayNum := 0
		if !date.IsZero() {
			dayNum = date.Day()
		}
		if dayNum <= 0 {
			if parsed := parseDayFromPath(resolved); parsed > 0 {
				dayNum = parsed
			}
		}
		if dayNum <= 0 {
			return
		}
		if _, exists := items[dayNum]; !exists {
			items[dayNum] = index.CollectionItem{Name: name, Resolved: resolved}
		}
	}
	if len(col.Days) > 0 {
		for _, day := range col.Days {
			addItem(day.Name, day.ID, day.Date)
		}
	}
	if len(col.Children) > 0 {
		for _, child := range col.Children {
			addItem(child.Name, child.ID, child.Day)
		}
	}
	if extra := m.calendarExtras[col.ID]; extra != nil {
		for day, item := range extra {
			if _, exists := items[day]; !exists {
				items[day] = item
			}
		}
	}
	if len(items) == 0 {
		return nil
	}
	keys := make([]int, 0, len(items))
	for day := range items {
		keys = append(keys, day)
	}
	sort.Ints(keys)
	result := make([]index.CollectionItem, 0, len(keys))
	for _, day := range keys {
		result = append(result, items[day])
	}
	return result
}

func (m *Model) handleCalendarMovement(msg tea.KeyMsg) (bool, tea.Cmd) {
	if !(key.Matches(msg, m.keys.MoveLeft) || key.Matches(msg, m.keys.MoveRight) ||
		key.Matches(msg, m.keys.MoveUp) || key.Matches(msg, m.keys.MoveDown)) {
		return false, nil
	}
	item, ok := m.selectedNavItem()
	if !ok || item.collection == nil || item.kind != RowKindDaily || item.folded {
		return false, nil
	}
	cal := m.ensureCalendar(item.collection)
	if cal == nil {
		return false, nil
	}
	next, cmd := cal.Update(msg)
	if model, ok := next.(*index.CalendarModel); ok {
		m.calendars[item.collection.ID] = model
	}
	m.refreshItems(item.collection.ID)
	return true, cmd
}

func (m *Model) handleCalendarFocusMsg(msg index.CalendarFocusMsg) {
	if msg.Direction == 0 {
		return
	}
	idx := m.list.Index()
	if idx < 0 {
		return
	}
	if msg.Direction < 0 && idx > 0 {
		m.list.Select(idx - 1)
	} else if msg.Direction > 0 && idx < len(m.list.Items())-1 {
		m.list.Select(idx + 1)
	}
	m.syncCalendarFocus()
}

func (m *Model) now() time.Time {
	if m.nowFn != nil {
		return m.nowFn()
	}
	return time.Now()
}

func (m *Model) syncCalendarFocus() {
	item, ok := m.selectedNavItem()
	var nextID string
	var nextCol *viewmodel.ParsedCollection
	var canFocus bool
	if ok && item.collection != nil && item.kind == RowKindDaily && !item.folded {
		nextID = item.collection.ID
		nextCol = item.collection
		canFocus = true
	}
	if m.activeCal == nextID {
		if canFocus && nextID != "" {
			if cal := m.ensureCalendar(nextCol); cal != nil && !cal.Focused() {
				cal.SetFocused(true)
			}
		}
		return
	}
	if prev := m.activeCal; prev != "" {
		if cal, ok := m.calendars[prev]; ok {
			cal.SetFocused(false)
		}
	}
	m.activeCal = nextID
	if !canFocus || nextID == "" {
		return
	}
	if cal := m.ensureCalendar(nextCol); cal != nil {
		cal.SetFocused(true)
	}
}

func (m *Model) selectedCalendarDay(col *viewmodel.ParsedCollection) (*viewmodel.ParsedCollection, bool) {
	if col == nil {
		return nil, false
	}
	cal := m.calendars[col.ID]
	if cal == nil {
		return nil, false
	}
	dayNum := cal.SelectedDay()
	if dayNum <= 0 {
		return nil, false
	}
	for _, child := range col.Children {
		if child == nil || child.Day.IsZero() {
			continue
		}
		if child.Day.Day() == dayNum {
			return child, true
		}
	}
	virtual := m.virtualDay(col, dayNum)
	if virtual == nil {
		return nil, false
	}
	return virtual, false
}

func (m *Model) virtualDay(col *viewmodel.ParsedCollection, day int) *viewmodel.ParsedCollection {
	if col == nil || day <= 0 {
		return nil
	}
	monthTime := m.monthTime(col)
	if monthTime.IsZero() {
		return nil
	}
	lastOfMonth := time.Date(monthTime.Year(), monthTime.Month()+1, 0, 0, 0, 0, 0, monthTime.Location())
	if day > lastOfMonth.Day() {
		return nil
	}
	dayTime := time.Date(monthTime.Year(), monthTime.Month(), day, 0, 0, 0, 0, monthTime.Location())
	dayName := dayTime.Format(dayLayout)
	return &viewmodel.ParsedCollection{
		ID:       fmt.Sprintf("%s/%s", col.ID, dayName),
		Name:     dayName,
		Type:     collection.TypeGeneric,
		Exists:   false,
		ParentID: col.ID,
		Depth:    col.Depth + 1,
		Priority: col.Priority + 1,
		SortKey:  strings.ToLower(dayName),
		Month:    monthTime,
		Day:      dayTime,
	}
}

func (m *Model) monthTime(col *viewmodel.ParsedCollection) time.Time {
	if col == nil {
		return time.Time{}
	}
	if !col.Month.IsZero() {
		return col.Month
	}
	if collection.IsMonthName(col.Name) {
		if t, err := time.Parse(monthLayout, col.Name); err == nil {
			return t
		}
	}
	return time.Time{}
}
