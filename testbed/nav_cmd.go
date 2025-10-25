package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/tui/components/collectionnav"
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
	collections := sampleCollections()
	nav := collectionnav.NewModel(collections)
	nav.SetFolded("Future", false)
	nav.SetFolded("Projects", true)
	nav.SetFolded("November 2025", true)
	model := &navTestModel{
		testbedModel: testbedModel{
			fullscreen: opts.full,
			maxWidth:   opts.width,
			maxHeight:  opts.height,
		},
		nav: nav,
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
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
			m.nav.Focus()
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

func (m *navTestModel) View() string {
	content := m.nav.View()
	if meta := m.metadataBar(); meta != "" {
		content = fmt.Sprintf("%s\n\n%s", content, meta)
	}
	frame := m.renderFrame(content)
	if m.fullscreen {
		return frame
	}
	return m.placeFrame(frame)
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

func sampleCollections() []*viewmodel.ParsedCollection {
	metas := []collection.Meta{
		{Name: "Inbox", Type: collection.TypeGeneric},
		{Name: "Future", Type: collection.TypeMonthly},
		{Name: "Future/December 2025", Type: collection.TypeGeneric},
		{Name: "October 2025", Type: collection.TypeDaily},
		{Name: "October 2025/October 5, 2025", Type: collection.TypeGeneric},
		{Name: "October 2025/October 12, 2025", Type: collection.TypeGeneric},
		{Name: "October 2025/October 22, 2025", Type: collection.TypeGeneric},
		{Name: "November 2025", Type: collection.TypeDaily},
		{Name: "November 2025/November 22, 2025", Type: collection.TypeGeneric},
		{Name: "Projects", Type: collection.TypeGeneric},
		{Name: "Projects/Side Quest", Type: collection.TypeGeneric},
		{Name: "Metrics", Type: collection.TypeTracking},
	}
	return viewmodel.BuildTree(metas, viewmodel.WithPriorities(map[string]int{
		"Inbox":  0,
		"Future": 10,
	}))
}
