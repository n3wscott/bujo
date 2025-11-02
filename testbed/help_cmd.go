package main

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/tui/components/command"
	dummyview "tableflip.dev/bujo/pkg/tui/components/dummy"
	"tableflip.dev/bujo/pkg/tui/components/help"
)

func newHelpCmd(opts *options) *cobra.Command {
	var useDummy bool

	cmd := &cobra.Command{
		Use:   "help",
		Short: "Render the help overlay component",
		RunE: func(cmd *cobra.Command, args []string) error {
			base := newTestbedModel(*opts)
			harness := &helpTestModel{
				testbedModel: base,
				useDummy:     useDummy,
			}
			harness.ensureSizing()
			program := tea.NewProgram(harness, tea.WithAltScreen())
			_, err := program.Run()
			return err
		},
	}

	cmd.Flags().BoolVar(&useDummy, "dummy", false, "render the dummy overlay")
	return cmd
}

type helpTestModel struct {
	testbedModel
	useDummy bool
	overlay  command.Overlay
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

	if m.overlay != nil {
		if next, cmd := m.overlay.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
			if next == nil {
				m.overlay = nil
			} else {
				m.overlay = next
			}
		}
	}

	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m *helpTestModel) View() (string, *tea.Cursor) {
	if m.overlay == nil {
		return m.composeView("help component unavailable", nil)
	}
	m.ensureSizing()
	content, cursor := m.overlay.View()
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
	if m.overlay == nil {
		if m.useDummy {
			m.overlay = dummyview.New(width, height)
		} else {
			m.overlay = help.New(width, height)
		}
		return
	}
	m.overlay.SetSize(width, height)
}
