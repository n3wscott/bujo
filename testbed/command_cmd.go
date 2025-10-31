package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/tui/components/command"
	"tableflip.dev/bujo/pkg/tui/events"
)

func newCommandCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "command",
		Short: "Preview the command bar and overlay host",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommandDemo(*opts)
		},
	}
	return cmd
}

func runCommandDemo(opts options) error {
	model := &commandTestModel{
		testbedModel: newTestbedModel(opts),
		command: command.NewModel(command.Options{
			ID:           events.ComponentID("CommandBar"),
			PromptPrefix: ":",
			StatusText:   "Press : to enter command • Tab toggles overlay • Esc cancels",
		}),
		hints: []string{
			"help",
			"open journal",
			"add task",
			"goto today",
			"toggle nav",
		},
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type commandTestModel struct {
	testbedModel
	command       *command.Model
	hints         []string
	lastSubmitted string
	contentWidth  int
	contentHeight int
}

func (m *commandTestModel) Init() tea.Cmd {
	if m.command == nil {
		return nil
	}
	return nil
}

func (m *commandTestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if _, cmd := m.testbedModel.Update(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}

	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		if m.command != nil {
			width, height := m.contentSize()
			m.contentWidth = width
			m.contentHeight = height
			m.command.SetSize(width, height)
		}
	case tea.KeyMsg:
		switch v.String() {
		case "tab":
			if m.command != nil {
				if m.command.HasOverlay() {
					m.command.CloseOverlay()
				} else {
					if cmd := m.openOverlay(); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			}
		}
	case events.CommandChangeMsg:
		if m.command != nil && v.Component == m.command.ID() {
			if v.Mode == events.CommandModeInput {
				m.command.SetStatus("typing: " + v.Value)
			}
		}
	case events.CommandSubmitMsg:
		if m.command != nil && v.Component == m.command.ID() {
			m.lastSubmitted = v.Value
			m.command.SetStatus("executed: " + v.Value)
		}
	}

	if m.command != nil {
		if _, cmd := m.command.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m *commandTestModel) View() (string, *tea.Cursor) {
	if m.command == nil {
		return m.composeView("command component unavailable", nil)
	}
	content := m.demoContent()
	m.command.SetContent(content, nil)
	view, cursor := m.command.View()
	return m.composeView(view, cursor)
}

func (m *commandTestModel) demoContent() string {
	width := m.contentWidth
	if width <= 0 {
		width = 60
	}
	height := m.contentHeight
	if height <= 0 {
		height = 12
	}

	header := fmt.Sprintf("Command Component Demo • Overlay active: %t", m.command != nil && m.command.HasOverlay())
	footer := fmt.Sprintf("Last submitted: %q", m.lastSubmitted)

	texture := make([]string, 0, height)
	palette := []string{"░", "▒", "▓", "█"}
	for row := 0; row < height-4; row++ {
		var builder strings.Builder
		for col := 0; col < width; col++ {
			glyph := palette[(row+col)%len(palette)]
			builder.WriteString(glyph)
		}
		texture = append(texture, builder.String())
	}

	overlay := []string{
		"Controls:",
		"  :         enter command mode",
		"  Tab       toggle suggestions overlay",
		"  Esc       cancel command or close overlay",
		"",
	}

	body := append([]string{header, strings.Repeat("─", width)}, overlay...)
	body = append(body, texture...)
	body = append(body, strings.Repeat("─", width), footer)
	return lipgloss.NewStyle().Width(width).Render(strings.Join(body, "\n"))
}

func (m *commandTestModel) openOverlay() tea.Cmd {
	if m.command == nil || len(m.hints) == 0 {
		return nil
	}
	overlay := newCompletionOverlay(events.ComponentID(m.command.ID()), m.hints)
	width := m.contentWidth
	if width <= 0 {
		width = 40
	}
	height := m.contentHeight
	if height <= 0 {
		height = 10
	}
	placement := command.OverlayPlacement{
		Width:      min(width/2, 36),
		Height:     min(height/2, 6),
		Horizontal: lipgloss.Left,
		Vertical:   lipgloss.Top,
	}
	return m.command.SetOverlay(overlay, placement)
}

type completionOverlay struct {
	target   events.ComponentID
	items    []string
	selected int
	width    int
	height   int
}

func newCompletionOverlay(target events.ComponentID, items []string) *completionOverlay {
	return &completionOverlay{
		target:   target,
		items:    append([]string(nil), items...),
		selected: 0,
	}
}

func (o *completionOverlay) Init() tea.Cmd { return nil }

func (o *completionOverlay) Update(msg tea.Msg) (command.Overlay, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyMsg:
		switch v.String() {
		case "esc":
			return nil, nil
		case "enter":
			if len(o.items) == 0 {
				return nil, nil
			}
			value := o.items[o.selected]
			return nil, events.CommandSubmitCmd(o.target, value)
		case "up", "k":
			if o.selected > 0 {
				o.selected--
			}
		case "down", "j":
			if o.selected < len(o.items)-1 {
				o.selected++
			}
		}
	}
	return o, nil
}

func (o *completionOverlay) View() (string, *tea.Cursor) {
	if o.width <= 0 {
		o.width = 20
	}
	if o.height <= 0 {
		o.height = min(len(o.items)+2, 8)
	}
	lines := make([]string, 0, o.height)
	for i := 0; i < len(o.items) && len(lines) < o.height-2; i++ {
		prefix := "  "
		if i == o.selected {
			prefix = "› "
		}
		lines = append(lines, prefix+o.items[i])
	}
	body := strings.Join(lines, "\n")
	view := lipgloss.NewStyle().
		Width(o.width).
		Height(o.height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("212")).
		Render(body)
	return view, nil
}

func (o *completionOverlay) SetSize(width, height int) {
	o.width = width
	o.height = height
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
