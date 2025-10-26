package main

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/events"
)

func newDetailCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detail",
		Short: "Preview the collection detail pane",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDetail(*opts)
		},
	}
	return cmd
}

func runDetail(opts options) error {
	detail := collectiondetail.NewModel(sampleDetailSections())
	detail.SetID(events.ComponentID("DetailPane"))
	model := &detailTestModel{
		testbedModel: newTestbedModel(opts),
		detail:       detail,
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type detailTestModel struct {
	testbedModel
	detail *collectiondetail.Model
}

func (m *detailTestModel) Init() tea.Cmd { return nil }

func (m *detailTestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if _, cmd := m.testbedModel.Update(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		width, height := m.contentSize()
		m.detail.SetSize(width, height)
	case tea.KeyMsg:
		if isDetailNavKey(msg.String()) {
			if cmd := m.detail.Focus(); cmd != nil {
				cmds = append(cmds, cmd)
			}
			m.testbedModel.SetFocus(true)
		}
	}
	if _, cmd := m.detail.Update(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m *detailTestModel) View() string {
	return m.composeView(m.detail.View())
}

func isDetailNavKey(key string) bool {
	switch key {
	case "up", "down", "k", "j", "pgup", "pgdown", "b", "f", "home", "end", "g", "G":
		return true
	default:
		return false
	}
}
