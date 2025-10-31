package main

import (
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/tui/components/index"
)

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
		testbedModel: newTestbedModel(opts),
		calendar:     cal,
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type calendarModel struct {
	testbedModel
	calendar *index.CalendarModel
}

func (m *calendarModel) Init() tea.Cmd { return nil }

func (m *calendarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if _, cmd := m.testbedModel.Update(msg); cmd != nil { //nolint:staticcheck // invoke embedded base update
		cmds = append(cmds, cmd)
	}
	if cmd := m.updateCalendar(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "left", "right", "up", "down", "h", "j", "k", "l":
			m.SetFocus(true)
		case "enter", " ":
			m.SetFocus(false)
		}
	}
	if focus, ok := msg.(index.CalendarFocusMsg); ok {
		m.SetFocus(false)
		if focus.Direction < 0 {
			m.SetFocus(false)
		}
	}
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m *calendarModel) View() (string, *tea.Cursor) {
	content := ""
	if m.calendar != nil {
		content = m.calendar.View()
	}
	return m.composeView(content, nil)
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
