package collectionnav

import (
	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"
)

// Model wraps a bubbles list for collection navigation.
type Model struct {
	list list.Model
}

// NewModel constructs the nav list with the provided collection names.
func NewModel(names []string) *Model {
	delegate := list.NewDefaultDelegate()
	l := list.New(itemsFromNames(names), delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	return &Model{list: l}
}

// SetItems replaces the rendered collections.
func (m *Model) SetItems(names []string) {
	m.list.SetItems(itemsFromNames(names))
}

// SetSize updates the list dimensions.
func (m *Model) SetSize(width, height int) {
	m.list.SetSize(width, height)
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// Update forwards Bubble Tea messages to the list.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the list.
func (m *Model) View() string {
	return m.list.View()
}

func itemsFromNames(names []string) []list.Item {
	items := make([]list.Item, 0, len(names))
	for _, name := range names {
		items = append(items, collectionItem{name: name})
	}
	return items
}

type collectionItem struct {
	name string
}

func (c collectionItem) Title() string       { return c.name }
func (collectionItem) Description() string   { return "" }
func (c collectionItem) FilterValue() string { return c.name }
