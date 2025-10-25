package main

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/tui/components/index"
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
		if key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))) {
			return m, tea.Quit
		}
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
		background = background.Background(lipgloss.Color("#003300"))
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

func newCalendarCmd(opts *options) *cobra.Command {
	var (
		monthFlag string
		selected  int
	)

	cmd := &cobra.Command{
		Use:   "calendar",
		Short: "Preview the calendar component",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCalendar(*opts, monthFlag, selected)
		},
	}

	cmd.Flags().StringVar(&monthFlag, "month", time.Now().Format("January 2006"), "month to render (e.g. \"March 2026\")")
	cmd.Flags().IntVar(&selected, "day", 0, "highlighted day number (optional)")
	return cmd
}

func runCalendar(opts options, month string, selectedDay int) error {
	cal := index.NewCalendarModel(month, selectedDay, time.Now())
	model := &calendarModel{
		testbedModel: testbedModel{
			fullscreen: opts.full,
			maxWidth:   opts.width,
			maxHeight:  opts.height,
		},
		calendar: cal,
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type calendarModel struct {
	testbedModel
	calendar *index.CalendarModel
}

func (m *calendarModel) Init() tea.Cmd {
	return nil
}

func (m *calendarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if _, cmd := m.testbedModel.Update(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if cmd := m.updateCalendar(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case tea.KeyMsg:
		if cmd := m.updateCalendar(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	default:
		if cmd := m.updateCalendar(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m *calendarModel) View() string {
	content := ""
	if m.calendar != nil {
		content = m.calendar.View()
	}
	if m.fullscreen {
		return content
	}
	width := clamp(m.maxWidth, 30, m.termWidth-4)
	height := clamp(m.maxHeight, 12, m.termHeight-4)
	frame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#39FF14")).
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)
	return lipgloss.Place(
		m.termWidth,
		m.termHeight,
		lipgloss.Center,
		lipgloss.Center,
		frame,
	)
}

func (m *calendarModel) updateCalendar(msg tea.Msg) tea.Cmd {
	if m.calendar == nil {
		return nil
	}
	next, cmd := m.calendar.Update(msg)
	if cal, ok := next.(*index.CalendarModel); ok {
		m.calendar = cal
	}
	return cmd
}
