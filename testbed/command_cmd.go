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
	suggestions := []command.SuggestionOption{
		{Name: "help", Description: "Show help information"},
		{Name: "open journal", Description: "Switch to journal view"},
		{Name: "add task", Description: "Create a new task entry"},
		{Name: "goto today", Description: "Jump to today"},
		{Name: "toggle nav", Description: "Toggle navigation focus"},
	}
	cmd := command.NewModel(command.Options{
		ID:           events.ComponentID("CommandBar"),
		PromptPrefix: ":",
		StatusText:   "Press : to enter command · suggestions appear as you type · Esc cancels",
	})
	cmd.SetSuggestions(suggestions)
	cmd.SetSuggestionLimit(8)
	model := &commandTestModel{
		testbedModel: newTestbedModel(opts),
		command:      cmd,
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type commandTestModel struct {
	testbedModel
	command       *command.Model
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
		// suggestions handled internally by the command component
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

	header := "Command Component Demo"
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

	instructions := []string{
		"Controls:",
		"  :         enter command mode",
		"  type      filter suggestions",
		"  Esc       cancel command input",
		"",
	}

	body := append([]string{header, strings.Repeat("─", width)}, instructions...)
	body = append(body, texture...)
	body = append(body, strings.Repeat("─", width), footer)
	return lipgloss.NewStyle().Width(width).Render(strings.Join(body, "\n"))
}
