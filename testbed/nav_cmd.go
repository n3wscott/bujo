package main

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/spf13/cobra"

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
	names := []string{"Inbox", "Projects", "Reference", "Archive"}
	nav := collectionnav.NewModel(names)
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
		if _, cmd := m.nav.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	default:
		if _, cmd := m.nav.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m *navTestModel) View() string {
	content := m.nav.View()
	frame := m.renderFrame(content)
	if m.fullscreen {
		return frame
	}
	return m.placeFrame(frame)
}
