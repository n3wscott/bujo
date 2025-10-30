package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/tui/components/collectionnav"
	"tableflip.dev/bujo/pkg/tui/events"
)

func newNavCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nav",
		Short: "Preview the collection navigation list",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNav(*opts)
		},
	}
	return cmd
}

func runNav(opts options) error {
	_, collections, err := loadCollectionsData(opts.real)
	if err != nil {
		return err
	}
	nav := collectionnav.NewModel(collections)
	nav.SetID(events.ComponentID("MainNav"))
	nav.SetFolded("Future", false)
	nav.SetFolded("Projects", true)
	nav.SetFolded("November 2025", true)
	model := &navTestModel{
		testbedModel: newTestbedModel(opts),
		nav:          nav,
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

type navTestModel struct {
	testbedModel
	nav *collectionnav.Model
}

func (m *navTestModel) Init() tea.Cmd { return nil }

func (m *navTestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if _, cmd := m.testbedModel.Update(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		width, height := m.contentSize()
		m.nav.SetSize(width, height)
		m.consumeNavCmd(msg, &cmds)
	case tea.KeyMsg:
		if isNavKey(msg.String()) {
			if cmd := m.nav.Focus(); cmd != nil {
				cmds = append(cmds, cmd)
			}
			m.testbedModel.SetFocus(true)
		}
		m.consumeNavCmd(msg, &cmds)
	case collectionnav.SelectionMsg:
		m.testbedModel.SetFocus(false)
	default:
		m.consumeNavCmd(msg, &cmds)
	}
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m *navTestModel) consumeNavCmd(msg tea.Msg, cmds *[]tea.Cmd) {
	if _, cmd := m.nav.Update(msg); cmd != nil {
		*cmds = append(*cmds, cmd)
	}
}

func (m *navTestModel) View() (string, *tea.Cursor) {
	content := m.nav.View()
	if meta := m.metadataBar(); meta != "" {
		content = fmt.Sprintf("%s\n\n%s", content, meta)
	}
	return m.composeView(content, nil)
}

func isNavKey(key string) bool {
	switch key {
	case "up", "down", "left", "right", "k", "j", "h", "l":
		return true
	default:
		return false
	}
}

func (m *navTestModel) metadataBar() string {
	col, kind, ok := m.nav.SelectedCollection()
	if !ok || col == nil {
		return "Selected: (none)"
	}
	parts := []string{
		fmt.Sprintf("Selected: %s", col.ID),
		fmt.Sprintf("Type: %s", col.Type),
		fmt.Sprintf("Row: %s", kind),
	}
	if !col.Month.IsZero() {
		parts = append(parts, fmt.Sprintf("Month: %s", col.Month.Format("Jan 2006")))
	}
	if !col.Day.IsZero() {
		parts = append(parts, fmt.Sprintf("Day: %s", col.Day.Format("Jan 2, 2006")))
	}
	if len(col.Children) > 0 {
		parts = append(parts, fmt.Sprintf("Children: %d", len(col.Children)))
	} else if len(col.Days) > 0 {
		parts = append(parts, fmt.Sprintf("Days: %d", len(col.Days)))
	}
	return strings.Join(parts, " | ")
}
