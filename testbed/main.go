package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/spf13/cobra"
)

type options struct {
	full   bool
	width  int
	height int
}

func main() {
	var opts options

	rootCmd := &cobra.Command{
		Use:   "testbed",
		Short: "Run the TUI testbed harness",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(opts)
		},
	}

	rootCmd.PersistentFlags().BoolVar(&opts.full, "full", false, "use the full terminal window")
	rootCmd.PersistentFlags().IntVar(&opts.width, "width", 80, "window width when not fullscreen")
	rootCmd.PersistentFlags().IntVar(&opts.height, "height", 20, "window height when not fullscreen")

	rootCmd.AddCommand(newCalendarCmd(&opts))

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(opts options) error {
	model := &testbedModel{
		fullscreen: opts.full,
		maxWidth:   opts.width,
		maxHeight:  opts.height,
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type testbedModel struct {
	fullscreen bool
	maxWidth   int
	maxHeight  int

	termWidth  int
	termHeight int

	focused bool
}

func (m *testbedModel) Init() tea.Cmd {
	return nil
}

func (m *testbedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.focused = !m.focused
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *testbedModel) SetFocus(f bool) {
	m.focused = f
}

func (m *testbedModel) View() string {
	if m.termWidth == 0 || m.termHeight == 0 {
		return "Resizing…"
	}

	content := lipgloss.NewStyle().
		Padding(1, 2).
		Render(
			"Testbed UI\n\n" +
				"Use this harness to iterate on components.\n\n" +
				"Press Tab to toggle focus, q to quit.",
		)

	frame := m.renderFrame(content)

	if m.fullscreen {
		return frame
	}

	return m.placeFrame(frame)
}

func (m *testbedModel) renderFrame(content string) string {
	borderStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	if m.focused {
		borderStyle = borderStyle.BorderForeground(lipgloss.Color("#39FF14"))
	} else {
		borderStyle = borderStyle.BorderForeground(lipgloss.Color("240"))
	}

	width := clamp(m.maxWidth, 20, m.termWidth-4)
	height := clamp(m.maxHeight, 10, m.termHeight-4)

	return borderStyle.
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)
}

func (m *testbedModel) placeFrame(frame string) string {
	background := lipgloss.NewStyle()
	if !m.focused {
		background = background.Background(lipgloss.Color("#39FF14"))
	}

	return lipgloss.Place(
		m.termWidth,
		m.termHeight,
		lipgloss.Center,
		lipgloss.Center,
		frame,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceStyle(background),
	)
}

func clamp(value, min, max int) int {
	if max <= 0 {
		return min
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
