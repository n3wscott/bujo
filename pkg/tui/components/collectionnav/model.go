package collectionnav

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/tui/components/index"
)

const (
	monthLayout = "January 2006"
	dayLayout   = "January 2, 2006"
)

// RowKind classifies how a collection row should render/behave.
type RowKind int

const (
	RowKindGeneric RowKind = iota
	RowKindMonthly
	RowKindDaily
	RowKindDay
	RowKindTracking
)

// String implements fmt.Stringer for debugging/rendering.
func (k RowKind) String() string {
	switch k {
	case RowKindMonthly:
		return "monthly"
	case RowKindDaily:
		return "daily"
	case RowKindDay:
		return "day"
	case RowKindTracking:
		return "tracking"
	default:
		return "generic"
	}
}

// Model wraps a bubbles list for collection navigation.
type Model struct {
	list    list.Model
	focused bool

	roots     []*viewmodel.ParsedCollection
	fold      map[string]bool
	calendars map[string]*index.CalendarModel
	nowFn     func() time.Time
}

type navDelegate struct {
	styles list.DefaultItemStyles
}

func newNavDelegate() navDelegate {
	base := list.NewDefaultDelegate()
	base.ShowDescription = false
	return navDelegate{styles: base.Styles}
}

func (navDelegate) Height() int  { return 1 }
func (navDelegate) Spacing() int { return 0 }
func (navDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d navDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	nav, ok := item.(navItem)
	if !ok {
		fmt.Fprint(w, item)
		return
	}
	view := nav.baseView()
	style := d.styles.NormalTitle
	if index == m.Index() && m.FilterState() != list.Filtering {
		style = d.styles.SelectedTitle
	} else if m.FilterState() == list.Filtering && m.FilterValue() == "" {
		style = d.styles.DimmedTitle
	}
	fmt.Fprint(w, style.Render(view))
}

// NewModel constructs the nav list for the provided collections.
func NewModel(collections []*viewmodel.ParsedCollection) *Model {
	delegate := newNavDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)

	m := &Model{
		list:      l,
		fold:      make(map[string]bool),
		calendars: make(map[string]*index.CalendarModel),
		nowFn:     time.Now,
	}
	m.SetCollections(collections)
	return m
}

// SetCollections replaces the rendered collections with a parsed tree.
func (m *Model) SetCollections(collections []*viewmodel.ParsedCollection) {
	m.roots = collections
	m.pruneFoldState()
	m.pruneCalendars()
	m.refreshItems("")
}

// SetSize updates the list dimensions.
func (m *Model) SetSize(width, height int) {
	m.list.SetSize(width, height)
}

// SetFolded pre-configures whether a collection is folded.
func (m *Model) SetFolded(id string, folded bool) {
	if id == "" {
		return
	}
	if m.fold == nil {
		m.fold = make(map[string]bool)
	}
	if folded {
		m.fold[id] = true
	} else {
		delete(m.fold, id)
	}
	m.refreshItems("")
}

// Focus marks the list as active.
func (m *Model) Focus() {
	m.focused = true
}

// Blur marks the list as inactive.
func (m *Model) Blur() {
	m.focused = false
}

// Focused reports whether the list currently has focus.
func (m *Model) Focused() bool {
	return m.focused
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// Update forwards Bubble Tea messages to the list and emits nav events.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var skipList bool
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if handled, cmd := m.handleCalendarMovement(keyMsg); handled {
			skipList = true
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	if !skipList {
		if next, cmd := m.list.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
			m.list = next
		} else {
			m.list = next
		}

		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if cmd := m.handleKeyMsg(keyMsg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	switch msg := msg.(type) {
	case index.CalendarFocusMsg:
		m.handleCalendarFocusMsg(msg)
	}

	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

// View renders the list.
func (m *Model) View() string {
	return m.list.View()
}

// SelectedCollection returns the currently highlighted collection and row kind.
func (m *Model) SelectedCollection() (*viewmodel.ParsedCollection, RowKind, bool) {
	item := m.list.SelectedItem()
	nav, ok := item.(navItem)
	if !ok {
		return nil, RowKindGeneric, false
	}
	col, kind, exists := m.selectionTarget(nav)
	if col == nil {
		return nil, RowKindGeneric, false
	}
	_ = exists
	return col, kind, true
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter", " ":
		if cmd := m.selectionCmd(); cmd != nil {
			return cmd
		}
	case "left", "h":
		if col := m.collapseSelected(); col != nil {
			m.refreshItems(col.ID)
			return nil
		}
	case "right", "l":
		if col := m.expandSelected(); col != nil {
			m.refreshItems(col.ID)
			return nil
		}
	}
	return nil
}

func (m *Model) selectionCmd() tea.Cmd {
	item, ok := m.selectedNavItem()
	if !ok {
		return nil
	}
	target, kind, exists := m.selectionTarget(item)
	if target == nil {
		return nil
	}
	m.Blur()
	return selectionCmd(target, kind, exists)
}

func (m *Model) collapseSelected() *viewmodel.ParsedCollection {
	item, ok := m.selectedNavItem()
	if !ok || !item.hasChildren {
		return nil
	}
	if m.fold[item.collection.ID] {
		return nil
	}
	m.fold[item.collection.ID] = true
	return item.collection
}

func (m *Model) expandSelected() *viewmodel.ParsedCollection {
	item, ok := m.selectedNavItem()
	if !ok || !item.hasChildren {
		return nil
	}
	if !m.fold[item.collection.ID] {
		return nil
	}
	delete(m.fold, item.collection.ID)
	return item.collection
}

func (m *Model) selectedNavItem() (navItem, bool) {
	item := m.list.SelectedItem()
	nav, ok := item.(navItem)
	if !ok || nav.collection == nil {
		return navItem{}, false
	}
	return nav, true
}

func (m *Model) refreshItems(preferredID string) {
	if preferredID == "" {
		preferredID = m.selectedID()
	}
	items := m.flattenCollections(m.roots, 0)
	m.list.SetItems(items)
	if preferredID != "" {
		m.selectByID(preferredID)
	}
}

func (m *Model) selectedID() string {
	item, ok := m.selectedNavItem()
	if !ok || item.collection == nil {
		return ""
	}
	return item.collection.ID
}

func (m *Model) selectByID(id string) {
	if id == "" {
		return
	}
	for idx, item := range m.list.Items() {
		nav, ok := item.(navItem)
		if !ok || nav.collection == nil {
			continue
		}
		if nav.collection.ID == id {
			m.list.Select(idx)
			return
		}
	}
}

func (m *Model) pruneFoldState() {
	if len(m.fold) == 0 {
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
	for id := range m.fold {
		if _, ok := valid[id]; !ok {
			delete(m.fold, id)
		}
	}
}

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
}

// SelectionMsg notifies parents that a collection row was activated.
type SelectionMsg struct {
	Collection *viewmodel.ParsedCollection
	Kind       RowKind
	Exists     bool
}

func selectionCmd(col *viewmodel.ParsedCollection, kind RowKind, exists bool) tea.Cmd {
	if col == nil {
		return nil
	}
	return func() tea.Msg {
		return SelectionMsg{Collection: col, Kind: kind, Exists: exists}
	}
}

type navItem struct {
	collection  *viewmodel.ParsedCollection
	depth       int
	kind        RowKind
	folded      bool
	hasChildren bool
	calendar    string
}

func (i navItem) Title() string { return i.baseView() }

func (i navItem) Description() string { return "" }

func (i navItem) FilterValue() string {
	if i.collection == nil {
		return ""
	}
	return i.collection.Name
}

func (i navItem) baseView() string {
	indent := strings.Repeat("  ", i.depth)
	lines := make([]string, 0, 1)
	label := i.collection.Name
	if i.calendar != "" {
		label += " ▾"
	} else if i.hasChildren {
		marker := "▾"
		if i.folded {
			marker = "▸"
		}
		label = label + " " + marker
	}
	lines = append(lines, fmt.Sprintf("%s%s", indent, label))
	if i.calendar != "" {
		block := strings.TrimRight(i.calendar, "\n")
		if block != "" {
			for _, line := range strings.Split(block, "\n") {
				if strings.TrimSpace(line) == "" {
					continue
				}
				lines = append(lines, fmt.Sprintf("%s%s", indent, line))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) flattenCollections(cols []*viewmodel.ParsedCollection, depth int) []list.Item {
	if len(cols) == 0 {
		return nil
	}
	items := make([]list.Item, 0, len(cols))
	for _, col := range cols {
		if col == nil {
			continue
		}
		kind := rowKindFor(col, depth)
		if kind == RowKindDay {
			continue
		}
		folded := m.fold[col.ID]
		item := navItem{
			collection:  col,
			depth:       depth,
			kind:        kind,
			folded:      folded,
			hasChildren: len(col.Children) > 0,
		}
		if kind == RowKindDaily && !folded {
			item.calendar = m.calendarBlock(col)
			items = append(items, item)
			continue
		}
		items = append(items, item)
		if len(col.Children) == 0 || folded {
			continue
		}
		children := m.flattenCollections(col.Children, depth+1)
		items = append(items, children...)
	}
	return items
}

func rowKindFor(col *viewmodel.ParsedCollection, depth int) RowKind {
	switch col.Type {
	case collection.TypeMonthly:
		return RowKindMonthly
	case collection.TypeDaily:
		return RowKindDaily
	case collection.TypeTracking:
		return RowKindTracking
	}
	if depth > 0 && !col.Day.IsZero() {
		return RowKindDay
	}
	return RowKindGeneric
}

func (m *Model) calendarBlock(col *viewmodel.ParsedCollection) string {
	cal := m.ensureCalendar(col)
	if cal == nil {
		return ""
	}
	lines := make([]string, 0, 1+len(cal.Rows()))
	if header := cal.Header(); header != nil {
		lines = append(lines, strings.TrimLeft(header.Text, " "))
	}
	for _, row := range cal.Rows() {
		if row == nil {
			continue
		}
		lines = append(lines, strings.TrimLeft(row.Text, " "))
	}
	return strings.Join(lines, "\n")
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
	cal.SetMonth(col.Name)
	cal.SetChildren(m.calendarChildren(col))
	return cal
}

func (m *Model) calendarChildren(col *viewmodel.ParsedCollection) []index.CollectionItem {
	if col == nil {
		return nil
	}
	if len(col.Days) > 0 {
		items := make([]index.CollectionItem, 0, len(col.Days))
		for _, day := range col.Days {
			items = append(items, index.CollectionItem{
				Name:     day.Name,
				Resolved: day.ID,
			})
		}
		return items
	}
	if len(col.Children) > 0 {
		items := make([]index.CollectionItem, 0, len(col.Children))
		for _, child := range col.Children {
			items = append(items, index.CollectionItem{
				Name:     child.Name,
				Resolved: child.ID,
			})
		}
		return items
	}
	return nil
}

func (m *Model) handleCalendarMovement(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch msg.String() {
	case "left", "right", "up", "down", "h", "j", "k", "l":
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
	default:
		return false, nil
	}
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
}

func (m *Model) now() time.Time {
	if m.nowFn != nil {
		return m.nowFn()
	}
	return time.Now()
}

func (m *Model) selectionTarget(item navItem) (*viewmodel.ParsedCollection, RowKind, bool) {
	if item.collection == nil {
		return nil, RowKindGeneric, false
	}
	if item.kind == RowKindDaily && !item.folded {
		if day, exists := m.selectedCalendarDay(item.collection); day != nil {
			return day, RowKindDay, exists
		}
	}
	return item.collection, item.kind, true
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
