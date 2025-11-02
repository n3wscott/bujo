package main

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/tui/components/help"
)

func newHelpCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "help",
		Short: "Render the help overlay component",
		RunE: func(cmd *cobra.Command, args []string) error {
			base := newTestbedModel(*opts)
			harness := &helpTestModel{
				testbedModel: base,
			}
			harness.ensureSizing()
			program := tea.NewProgram(harness, tea.WithAltScreen())
			_, err := program.Run()
			return err
		},
	}
	return cmd
}

type helpTestModel struct {
	testbedModel
	view *help.Model
}

func (m *helpTestModel) Init() tea.Cmd {
	m.ensureSizing()
	return nil
}

func (m *helpTestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if _, cmd := m.testbedModel.Update(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}

	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.ensureSizing()
	case tea.KeyMsg:
		switch v.String() {
		case "esc", ":":
			cmds = append(cmds, tea.Quit)
		}
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

func (m *helpTestModel) View() (string, *tea.Cursor) {
	if m.view == nil {
		return m.composeView("help component unavailable", nil)
	}
	m.ensureSizing()
	content, cursor := m.view.View()
	return m.composeView(content, cursor)
}

func (m *helpTestModel) ensureSizing() {
	m.ensureLayout()
	width := m.innerWidth
	height := m.innerHeight
	if width <= 0 {
		width = 72
	}
	if height <= 0 {
		height = 18
	}
	if m.view == nil {
		m.view = help.New(width, height)
		return
	}
	m.view.SetSize(width, height)
}
