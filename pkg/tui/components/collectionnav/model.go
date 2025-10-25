package collectionnav

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/collection/viewmodel"
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

	roots []*viewmodel.ParsedCollection
	fold  map[string]bool
}

// NewModel constructs the nav list for the provided collections.
func NewModel(collections []*viewmodel.ParsedCollection) *Model {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)

	m := &Model{
		list: l,
		fold: make(map[string]bool),
	}
	m.SetCollections(collections)
	return m
}

// SetCollections replaces the rendered collections with a parsed tree.
func (m *Model) SetCollections(collections []*viewmodel.ParsedCollection) {
	m.roots = collections
	m.pruneFoldState()
	m.refreshItems(m.selectedID())
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
		if m.fold[id] {
			return
		}
		m.fold[id] = true
	} else {
		if !m.fold[id] {
			return
		}
		delete(m.fold, id)
	}
	m.refreshItems(m.selectedID())
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
	item, ok := m.selectedItem()
	if !ok {
		return nil, RowKindGeneric, false
	}
	return item.collection, item.kind, true
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter", " ":
		if cmd := m.selectionCmd(); cmd != nil {
			return cmd
		}
	case "left", "h":
		if m.collapseSelected() {
			m.refreshItems(m.selectedID())
			return nil
		}
	case "right", "l":
		if m.expandSelected() {
			m.refreshItems(m.selectedID())
			return nil
		}
	}
	return nil
}

func (m *Model) selectionCmd() tea.Cmd {
	item, ok := m.selectedItem()
	if !ok {
		return nil
	}
	m.Blur()
	return selectionCmd(item.collection, item.kind)
}

func (m *Model) collapseSelected() bool {
	item, ok := m.selectedItem()
	if !ok || !item.hasChildren {
		return false
	}
	if m.fold[item.collection.ID] {
		return false
	}
	m.fold[item.collection.ID] = true
	return true
}

func (m *Model) expandSelected() bool {
	item, ok := m.selectedItem()
	if !ok || !item.hasChildren {
		return false
	}
	if !m.fold[item.collection.ID] {
		return false
	}
	delete(m.fold, item.collection.ID)
	return true
}

func (m *Model) refreshItems(selectedID string) {
	items := flattenCollections(m.roots, m.fold, 0)
	m.list.SetItems(items)
	if selectedID != "" {
		m.selectByID(selectedID)
	}
}

func (m *Model) selectedID() string {
	item, ok := m.selectedItem()
	if !ok {
		return ""
	}
	return item.collection.ID
}

func (m *Model) selectByID(id string) {
	for idx, item := range m.list.Items() {
		nav, ok := item.(navItem)
		if !ok {
			continue
		}
		if nav.collection.ID == id {
			m.list.Select(idx)
			return
		}
	}
}

func (m *Model) selectedItem() (navItem, bool) {
	idx := m.list.Index()
	if idx < 0 {
		return navItem{}, false
	}
	items := m.list.Items()
	if idx >= len(items) {
		return navItem{}, false
	}
	item, ok := items[idx].(navItem)
	return item, ok
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

// SelectionMsg notifies parents that a collection row was activated.
type SelectionMsg struct {
	Collection *viewmodel.ParsedCollection
	Kind       RowKind
}

func selectionCmd(col *viewmodel.ParsedCollection, kind RowKind) tea.Cmd {
	if col == nil {
		return nil
	}
	return func() tea.Msg {
		return SelectionMsg{Collection: col, Kind: kind}
	}
}

type navItem struct {
	collection  *viewmodel.ParsedCollection
	depth       int
	kind        RowKind
	folded      bool
	hasChildren bool
}

func (i navItem) Title() string {
	indent := strings.Repeat("  ", i.depth)
	if i.hasChildren {
		marker := "▾"
		if i.folded {
			marker = "▸"
		}
		return fmt.Sprintf("%s%s %s", indent, marker, i.collection.Name)
	}
	return fmt.Sprintf("%s%s", indent, i.collection.Name)
}

func (i navItem) Description() string {
	return string(i.collection.Type)
}

func (i navItem) FilterValue() string {
	return i.collection.Name
}

func flattenCollections(cols []*viewmodel.ParsedCollection, fold map[string]bool, depth int) []list.Item {
	if len(cols) == 0 {
		return nil
	}
	items := make([]list.Item, 0, len(cols))
	for _, col := range cols {
		if col == nil {
			continue
		}
		item := navItem{
			collection:  col,
			depth:       depth,
			kind:        rowKindFor(col, depth),
			folded:      fold[col.ID],
			hasChildren: len(col.Children) > 0,
		}
		items = append(items, item)
		if len(col.Children) == 0 || fold[col.ID] {
			continue
		}
		children := flattenCollections(col.Children, fold, depth+1)
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
