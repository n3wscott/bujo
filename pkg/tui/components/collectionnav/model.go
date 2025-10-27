package collectionnav

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/tui/components/index"
	"tableflip.dev/bujo/pkg/tui/events"
	"tableflip.dev/bujo/pkg/tui/uiutil"
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

// SelectionMsg aliases the shared select event for backward compatibility.
type SelectionMsg = events.CollectionSelectMsg

// Model wraps a bubbles list for collection navigation.
type Model struct {
	list    list.Model
	focused bool

	roots     []*viewmodel.ParsedCollection
	metas     []collection.Meta
	index     map[string]*viewmodel.ParsedCollection
	fold      map[string]bool
	calendars map[string]*index.CalendarModel
	activeCal string
	nowFn     func() time.Time

	id            events.ComponentID
	lastHighlight string
}

type navDelegate struct {
	styles           list.DefaultItemStyles
	selectedActive   lipgloss.Style
	selectedInactive lipgloss.Style
	focused          func() bool
}

func newNavDelegate() navDelegate {
	base := list.NewDefaultDelegate()
	base.ShowDescription = false
	selected := base.Styles.SelectedTitle.Copy()
	normalFG := base.Styles.NormalTitle.GetForeground()
	if normalFG == nil {
		normalFG = selected.GetForeground()
	}
	inactive := selected.Copy().Foreground(normalFG)
	return navDelegate{
		styles:           base.Styles,
		selectedActive:   selected,
		selectedInactive: inactive,
	}
}

func newNavDelegateWithFocus(m *Model) navDelegate {
	delegate := newNavDelegate()
	if m != nil {
		delegate.focused = func() bool {
			return m.focused
		}
	}
	return delegate
}

func (d navDelegate) Height() int  { return 1 }
func (d navDelegate) Spacing() int { return 0 }
func (d navDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
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
	focused := true
	if d.focused != nil {
		focused = d.focused()
	}
	if index == m.Index() && m.FilterState() != list.Filtering {
		if focused {
			style = d.selectedActive
		} else {
			style = d.selectedInactive
		}
	} else if m.FilterState() == list.Filtering && m.FilterValue() == "" {
		style = d.styles.DimmedTitle
	}
	fmt.Fprint(w, style.Render(view))
}

// NewModel constructs the nav list for the provided collections.
func NewModel(collections []*viewmodel.ParsedCollection) *Model {
	m := &Model{
		fold:      make(map[string]bool),
		calendars: make(map[string]*index.CalendarModel),
		index:     make(map[string]*viewmodel.ParsedCollection),
		nowFn:     time.Now,
		id:        events.ComponentID("collectionnav"),
	}
	delegate := newNavDelegateWithFocus(m)
	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.SetShowFilter(false)
	m.list = l
	m.SetCollections(collections)
	m.list.SetDelegate(delegate)
	return m
}

// SetID overrides the emitted ComponentID.
func (m *Model) SetID(id events.ComponentID) {
	if id == "" {
		m.id = events.ComponentID("collectionnav")
		return
	}
	m.id = id
}

// ID returns the component identifier.
func (m *Model) ID() events.ComponentID {
	return m.id
}

// SetCollections replaces the rendered collections with a parsed tree.
func (m *Model) SetCollections(collections []*viewmodel.ParsedCollection) {
	m.setCollectionsInternal(collections, true, "")
}

func (m *Model) setCollectionsInternal(collections []*viewmodel.ParsedCollection, rebuildMetas bool, preferredID string) {
	m.roots = collections
	if rebuildMetas {
		m.metas = flattenMetas(collections)
	}
	m.rebuildIndex()
	m.pruneFoldState()
	m.pruneCalendars()
	m.lastHighlight = ""
	m.refreshItems(preferredID)
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
func (m *Model) Focus() tea.Cmd {
	if m.focused {
		return nil
	}
	m.focused = true
	m.list.SetDelegate(newNavDelegateWithFocus(m))
	return events.FocusCmd(m.id)
}

// Blur marks the list as inactive.
func (m *Model) Blur() tea.Cmd {
	if !m.focused {
		return nil
	}
	m.focused = false
	m.list.SetDelegate(newNavDelegateWithFocus(m))
	return events.BlurCmd(m.id)
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
		if !m.focused {
			return m, nil
		}
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
		m.syncCalendarFocus()
	}

	if cmd := m.highlightCmd(); cmd != nil {
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case index.CalendarFocusMsg:
		m.handleCalendarFocusMsg(msg)
	case events.CollectionChangeMsg:
		if m.handleCollectionChange(msg) {
			if cmd := m.highlightCmd(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	case events.CollectionOrderMsg:
		if m.applyCollectionOrder(msg.Order) {
			if cmd := m.highlightCmd(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
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
	case "left", "h", "[":
		if col := m.collapseSelected(); col != nil {
			m.refreshItems(col.ID)
			return nil
		}
	case "right", "l", "]":
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
	var cmds []tea.Cmd
	if blur := m.Blur(); blur != nil {
		cmds = append(cmds, blur)
	}
	if selectCmd := selectCmd(m.id, target, kind, exists); selectCmd != nil {
		cmds = append(cmds, selectCmd)
	}
	return tea.Batch(cmds...)
}

func (m *Model) highlightCmd() tea.Cmd {
	if !m.focused {
		return nil
	}
	item, ok := m.selectedNavItem()
	if !ok {
		if m.lastHighlight != "" {
			m.lastHighlight = ""
		}
		return nil
	}
	target, kind, _ := m.selectionTarget(item)
	if target == nil {
		if m.lastHighlight != "" {
			m.lastHighlight = ""
		}
		return nil
	}
	if target.ID == m.lastHighlight {
		return nil
	}
	m.lastHighlight = target.ID
	return highlightCmd(m.id, target, kind)
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
		m.selectListItemByID(preferredID)
	}
	m.syncCalendarFocus()
}

func (m *Model) selectedID() string {
	item, ok := m.selectedNavItem()
	if !ok || item.collection == nil {
		return ""
	}
	return item.collection.ID
}

func (m *Model) selectListItemByID(id string) bool {
	if id == "" {
		return false
	}
	for idx, item := range m.list.Items() {
		nav, ok := item.(navItem)
		if !ok || nav.collection == nil {
			continue
		}
		if nav.collection.ID == id {
			if idx == m.list.Index() {
				return false
			}
			m.list.Select(idx)
			return true
		}
	}
	return false
}

func (m *Model) selectByName(name string) bool {
	if name == "" {
		return false
	}
	lower := strings.ToLower(name)
	for idx, item := range m.list.Items() {
		nav, ok := item.(navItem)
		if !ok || nav.collection == nil {
			continue
		}
		if strings.ToLower(nav.collection.Name) == lower {
			if idx == m.list.Index() {
				return false
			}
			m.list.Select(idx)
			return true
		}
	}
	return false
}

// SelectCollection moves the cursor to the referenced collection (by ID or
// name) and emits a highlight event if the selection changed.
func (m *Model) SelectCollection(ref events.CollectionRef) tea.Cmd {
	if !m.ensureSelection(ref) {
		return nil
	}
	m.syncCalendarFocus()
	return m.highlightCmd()
}

func (m *Model) ensureSelection(ref events.CollectionRef) bool {
	var changed bool
	if ref.ID != "" {
		changed = m.selectListItemByID(ref.ID)
	}
	if !changed && ref.Name != "" {
		changed = m.selectByName(ref.Name)
	}
	if m.applyCalendarSelection(ref) {
		changed = true
	}
	return changed
}

func (m *Model) applyCalendarSelection(ref events.CollectionRef) bool {
	parentID := ref.ParentID
	if parentID == "" {
		parentID = parentFromPath(ref.ID)
	}
	if parentID == "" {
		return false
	}
	day := 0
	if !ref.Day.IsZero() {
		day = ref.Day.Day()
	}
	if day == 0 {
		day = uiutil.ParseDayNumber(parentLabel(parentID), ref.Name)
	}
	if day == 0 {
		day = parseDayFromPath(ref.ID)
	}
	if day == 0 {
		return false
	}

	if m.fold[parentID] {
		delete(m.fold, parentID)
		m.refreshItems(parentID)
	}

	m.selectListItemByID(parentID)

	parent := m.lookup(parentID)
	if parent == nil {
		parent = &viewmodel.ParsedCollection{
			ID:   parentID,
			Name: parentLabel(parentID),
			Type: collection.TypeDaily,
		}
	}

	cal := m.ensureCalendar(parent)
	if cal == nil {
		return false
	}
	if cal.SelectedDay() == day {
		return true
	}
	cal.SetSelected(day)
	return true
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

func (m *Model) rebuildIndex() {
	if m.index == nil {
		m.index = make(map[string]*viewmodel.ParsedCollection)
	} else {
		for k := range m.index {
			delete(m.index, k)
		}
	}
	var stack []*viewmodel.ParsedCollection
	stack = append(stack, m.roots...)
	for len(stack) > 0 {
		last := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if last == nil {
			continue
		}
		m.index[last.ID] = last
		if len(last.Children) > 0 {
			stack = append(stack, last.Children...)
		}
	}
}

func (m *Model) lookup(id string) *viewmodel.ParsedCollection {
	if id == "" {
		return nil
	}
	if m.index == nil {
		return nil
	}
	return m.index[id]
}

func parentFromPath(path string) string {
	if path == "" {
		return ""
	}
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[:idx]
	}
	return ""
}

func parentLabel(path string) string {
	if path == "" {
		return ""
	}
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

func parseDayFromPath(path string) int {
	if path == "" {
		return 0
	}
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		parent := path[:idx]
		child := path[idx+1:]
		return uiutil.ParseDayNumber(parentLabel(parent), child)
	}
	return 0
}

func selectCmd(component events.ComponentID, col *viewmodel.ParsedCollection, kind RowKind, exists bool) tea.Cmd {
	if col == nil {
		return nil
	}
	return func() tea.Msg {
		return events.CollectionSelectMsg{
			Component:  component,
			Collection: events.RefFromParsed(col),
			RowKind:    kind.String(),
			Exists:     exists,
		}
	}
}

func highlightCmd(component events.ComponentID, col *viewmodel.ParsedCollection, kind RowKind) tea.Cmd {
	if col == nil {
		return nil
	}
	return func() tea.Msg {
		return events.CollectionHighlightMsg{
			Component:  component,
			Collection: events.RefFromParsed(col),
			RowKind:    kind.String(),
		}
	}
}

type navItem struct {
	collection  *viewmodel.ParsedCollection
	depth       int
	kind        RowKind
	folded      bool
	hasChildren bool
	calendar    *index.CalendarModel
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
	if i.calendar != nil {
		label += " ▾"
	} else if i.hasChildren {
		marker := "▾"
		if i.folded {
			marker = "▸"
		}
		label = label + " " + marker
	}
	lines = append(lines, fmt.Sprintf("%s%s", indent, label))
	if i.calendar != nil {
		block := strings.TrimRight(i.calendar.View(), "\n")
		for _, line := range strings.Split(block, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s%s", indent, strings.TrimLeft(line, " ")))
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
			item.calendar = m.ensureCalendar(col)
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

func (m *Model) handleCollectionChange(msg events.CollectionChangeMsg) bool {
	m.ensureMetaSnapshot()
	var changed bool
	switch msg.Action {
	case events.ChangeCreate:
		changed = m.addCollectionMeta(msg.Current)
	case events.ChangeUpdate:
		changed = m.updateCollectionMeta(msg.Current, msg.Previous)
	case events.ChangeDelete:
		changed = m.removeCollectionMeta(msg.Current, msg.Previous)
	default:
		return false
	}
	if !changed {
		return false
	}
	m.reloadCollectionsFromMetas(m.selectedID())
	return true
}

func (m *Model) applyCollectionOrder(order []string) bool {
	if len(order) == 0 || len(m.roots) == 0 {
		return false
	}
	index := orderIndexMap(order)
	if !reorderParsedCollections(m.roots, index) {
		return false
	}
	m.refreshItems("")
	return true
}

func (m *Model) ensureMetaSnapshot() {
	if len(m.metas) > 0 || len(m.roots) == 0 {
		return
	}
	m.metas = flattenMetas(m.roots)
}

func (m *Model) reloadCollectionsFromMetas(preferredID string) {
	if len(m.metas) == 0 {
		m.setCollectionsInternal(nil, false, preferredID)
		return
	}
	m.setCollectionsInternal(viewmodel.BuildTree(m.metas), false, preferredID)
}

func (m *Model) addCollectionMeta(ref events.CollectionRef) bool {
	name := fullCollectionName(ref)
	if name == "" {
		return false
	}
	return m.upsertMeta(name, normalizeType(ref.Type))
}

func (m *Model) updateCollectionMeta(current events.CollectionRef, previous *events.CollectionRef) bool {
	currName := fullCollectionName(current)
	prevName := currName
	if previous != nil {
		if name := fullCollectionName(*previous); name != "" {
			prevName = name
		}
	}
	if prevName == "" {
		prevName = currName
	}
	if prevName == "" {
		return false
	}
	if currName == "" {
		currName = prevName
	}
	idx := metaIndex(m.metas, prevName)
	if idx < 0 {
		return m.upsertMeta(currName, normalizeType(current.Type))
	}
	changed := false
	if m.metas[idx].Name != currName {
		m.metas[idx].Name = currName
		changed = true
	}
	if typ := normalizeType(current.Type); typ != "" && m.metas[idx].Type != typ {
		m.metas[idx].Type = typ
		changed = true
	}
	return changed
}

func (m *Model) removeCollectionMeta(current events.CollectionRef, previous *events.CollectionRef) bool {
	target := fullCollectionName(current)
	if target == "" && previous != nil {
		target = fullCollectionName(*previous)
	}
	if target == "" {
		return false
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	filtered := m.metas[:0]
	removed := false
	for _, meta := range m.metas {
		if meta.Name == target || strings.HasPrefix(meta.Name, target+"/") {
			removed = true
			continue
		}
		filtered = append(filtered, meta)
	}
	if removed {
		m.metas = filtered
	}
	return removed
}

func (m *Model) upsertMeta(name string, typ collection.Type) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if typ == "" {
		typ = collection.TypeGeneric
	}
	if idx := metaIndex(m.metas, name); idx >= 0 {
		if m.metas[idx].Type == typ {
			return false
		}
		m.metas[idx].Type = typ
		return true
	}
	m.metas = append(m.metas, collection.Meta{Name: name, Type: typ})
	return true
}

func flattenMetas(nodes []*viewmodel.ParsedCollection) []collection.Meta {
	if len(nodes) == 0 {
		return nil
	}
	metas := make([]collection.Meta, 0, len(nodes))
	var walk func(list []*viewmodel.ParsedCollection)
	walk = func(list []*viewmodel.ParsedCollection) {
		for _, node := range list {
			if node == nil {
				continue
			}
			metas = append(metas, collection.Meta{Name: node.ID, Type: node.Type})
			if len(node.Children) > 0 {
				walk(node.Children)
			}
		}
	}
	walk(nodes)
	return metas
}

func metaIndex(metas []collection.Meta, name string) int {
	for idx, meta := range metas {
		if meta.Name == name {
			return idx
		}
	}
	return -1
}

func fullCollectionName(ref events.CollectionRef) string {
	if ref.ID != "" {
		return strings.TrimSpace(ref.ID)
	}
	switch {
	case ref.ParentID != "" && ref.Name != "":
		return strings.TrimSpace(fmt.Sprintf("%s/%s", strings.TrimSuffix(ref.ParentID, "/"), ref.Name))
	case ref.Name != "":
		return strings.TrimSpace(ref.Name)
	default:
		return ""
	}
}

func normalizeType(typ collection.Type) collection.Type {
	if typ == "" {
		return collection.TypeGeneric
	}
	return typ
}

func orderIndexMap(order []string) map[string]int {
	index := make(map[string]int, len(order))
	for i, id := range order {
		key := strings.ToLower(strings.TrimSpace(id))
		if key == "" {
			continue
		}
		if _, exists := index[key]; !exists {
			index[key] = i
		}
	}
	return index
}

func reorderParsedCollections(nodes []*viewmodel.ParsedCollection, order map[string]int) bool {
	if len(nodes) == 0 {
		return false
	}
	changed := reorderNodeSlice(nodes, order)
	for _, node := range nodes {
		if node == nil || len(node.Children) == 0 {
			continue
		}
		if reorderParsedCollections(node.Children, order) {
			changed = true
		}
	}
	return changed
}

func reorderNodeSlice(nodes []*viewmodel.ParsedCollection, order map[string]int) bool {
	if len(nodes) <= 1 {
		return false
	}
	before := make([]string, len(nodes))
	for i, node := range nodes {
		if node != nil {
			before[i] = node.ID
		}
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		return collectionOrderIndex(nodes[i], order) < collectionOrderIndex(nodes[j], order)
	})
	for i, node := range nodes {
		id := ""
		if node != nil {
			id = node.ID
		}
		if before[i] != id {
			return true
		}
	}
	return false
}

func collectionOrderIndex(node *viewmodel.ParsedCollection, order map[string]int) int {
	if node == nil {
		return len(order) * 2
	}
	key := strings.ToLower(strings.TrimSpace(node.ID))
	if idx, ok := order[key]; ok {
		return idx
	}
	return len(order)*2 + node.Priority
}
