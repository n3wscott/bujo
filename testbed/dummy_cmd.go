package main

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/spf13/cobra"

	dummyview "tableflip.dev/bujo/pkg/tui/components/dummy"
)

func newDummyCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dummy",
		Short: "Render the dummy S-line component",
		RunE: func(cmd *cobra.Command, args []string) error {
			model := &dummyModel{
				testbedModel: newTestbedModel(*opts),
			}
			program := tea.NewProgram(model, tea.WithAltScreen())
			_, err := program.Run()
			return err
		},
	}
	return cmd
}

type dummyModel struct {
	testbedModel
	view *dummyview.Model
}

func (m *dummyModel) Init() tea.Cmd {
	m.ensureSizing()
	return nil
}

func (m *dummyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if _, cmd := m.testbedModel.Update(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}

	switch msg.(type) {
	case tea.WindowSizeMsg:
		m.ensureSizing()
	}

	if m.view != nil {
		if _, cmd := m.view.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m *dummyModel) View() (string, *tea.Cursor) {
	m.ensureSizing()
	if m.view == nil {
		return m.composeView("dummy component unavailable", nil)
	}
	content, cursor := m.view.View()
	return m.composeView(content, cursor)
}

func (m *dummyModel) ensureSizing() {
	m.ensureLayout()
	width := m.innerWidth
	height := m.innerHeight
	if width <= 0 {
		width = 2
	}
	if height <= 0 {
		height = 1
	}
	if m.view == nil {
		m.view = dummyview.New(width, height)
		return
	}
	m.view.SetSize(width, height)
}
