package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/spf13/cobra"

	"tableflip.dev/bujo/pkg/tui/components/eventviewer"
	"tableflip.dev/bujo/pkg/tui/events"
)

type options struct {
	full   bool
	width  int
	height int
	real   bool
	hold   int
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
	rootCmd.PersistentFlags().BoolVar(&opts.real, "real", false, "load data from the real journal database")
	rootCmd.PersistentFlags().IntVar(&opts.hold, "hold", 0, "number of bullets to hold back and replay via events")

	rootCmd.AddCommand(newCalendarCmd(&opts))
	rootCmd.AddCommand(newNavCmd(&opts))
	rootCmd.AddCommand(newDetailCmd(&opts))
	rootCmd.AddCommand(newJournalCmd(&opts))
	rootCmd.AddCommand(newAddCmd(&opts))
	rootCmd.AddCommand(newCommandCmd(&opts))

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(opts options) error {
	base := newTestbedModel(opts)
	p := tea.NewProgram(&base, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type testbedModel struct {
	fullscreen bool
	maxWidth   int
	maxHeight  int

	termWidth  int
	termHeight int

	focused    bool
	focusOwner events.ComponentID

	events *eventviewer.Model

	frameWidth  int
	frameHeight int
	innerWidth  int
	innerHeight int
	eventHeight int
	layoutDirty bool
}

func newTestbedModel(opts options) testbedModel {
	return testbedModel{
		fullscreen:  opts.full,
		maxWidth:    opts.width,
		maxHeight:   opts.height,
		focused:     false,
		events:      eventviewer.NewModel(400),
		layoutDirty: true,
	}
}

func (m *testbedModel) Init() tea.Cmd { return nil }

func (m *testbedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.recordEvent(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.layoutDirty = true
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case events.FocusMsg:
		m.focused = true
		m.focusOwner = msg.Component
	case events.BlurMsg:
		if m.focusOwner == msg.Component || m.focusOwner == "" {
			m.focused = false
			m.focusOwner = ""
		}
	}

	if m.events != nil {
		m.events.Update(msg)
	}

	return m, nil
}

func (m *testbedModel) SetFocus(f bool) {
	m.focused = f
	if !f {
		m.focusOwner = ""
	}
}

func (m *testbedModel) View() (string, *tea.Cursor) {
	content := lipgloss.NewStyle().
		Padding(1, 2).
		Render(
			"Testbed UI\n\n" +
				"Use this harness to iterate on components.\n\n" +
				"Press Tab to toggle focus, q to quit.",
		)
	return m.composeView(content, nil)
}

func (m *testbedModel) composeView(content string, cursor *tea.Cursor) (string, *tea.Cursor) {
	if m.termWidth == 0 || m.termHeight == 0 {
		return "Resizing…", nil
	}
	m.ensureLayout()

	frame, cursor := m.renderFrame(content, cursor)
	frameBlock, cursor := m.placeFrame(frame, cursor)

	if events := m.renderEvents(); events != "" {
		var gap string
		if frameGap > 0 {
			gap = lipgloss.NewStyle().
				Width(m.termWidth).
				Height(frameGap).
				Render(strings.Repeat(" ", m.termWidth))
		}
		if gap != "" {
			frameBlock = lipgloss.JoinVertical(lipgloss.Left, frameBlock, gap, events)
		} else {
			frameBlock = lipgloss.JoinVertical(lipgloss.Left, frameBlock, events)
		}
	}

	return frameBlock, cursor
}

func (m *testbedModel) renderFrame(content string, cursor *tea.Cursor) (string, *tea.Cursor) {
	m.ensureLayout()

	borderStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	if m.focused {
		borderStyle = borderStyle.BorderForeground(lipgloss.Color("#39FF14"))
	} else {
		borderStyle = borderStyle.BorderForeground(lipgloss.Color("240"))
	}

	width := m.frameWidth
	height := m.frameHeight
	innerWidth := m.innerWidth
	innerHeight := m.innerHeight

	contentView := lipgloss.NewStyle().
		Padding(0).
		Width(innerWidth).
		Height(innerHeight).
		Align(lipgloss.Left, lipgloss.Top).
		Render(content)

	contentStyle := lipgloss.NewStyle().
		Width(innerWidth).
		Height(innerHeight).
		Align(lipgloss.Left, lipgloss.Top)

	frame := borderStyle.Width(width).Height(height).Render(contentStyle.Render(contentView))
	if cursor == nil {
		return frame, nil
	}
	return frame, offsetCursor(cursor, 1, 1)
}

func (m *testbedModel) renderEvents() string {
	if m.events == nil || m.eventHeight == 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Width(m.termWidth).
		Height(m.eventHeight).
		Align(lipgloss.Left, lipgloss.Bottom).
		Render(m.events.View())
}

func (m *testbedModel) placeFrame(frame string, cursor *tea.Cursor) (string, *tea.Cursor) {
	background := lipgloss.NewStyle()
	if !m.focused {
		background = background.Background(lipgloss.Color("#39FF14"))
	}

	height := max(1, m.termHeight-m.eventHeight-frameGap)

	offsetX := 0
	frameWidth := lipgloss.Width(frame)
	if frameWidth < m.termWidth {
		offsetX = (m.termWidth - frameWidth) / 2
	}
	placed := lipgloss.Place(
		m.termWidth,
		height,
		lipgloss.Center,
		lipgloss.Top,
		frame,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceStyle(background),
	)
	if cursor == nil {
		return placed, nil
	}
	return placed, offsetCursor(cursor, offsetX, 0)
}

func (m *testbedModel) contentSize() (int, int) {
	m.ensureLayout()
	return m.innerWidth, m.innerHeight
}

func (m *testbedModel) ensureLayout() {
	if m.termWidth == 0 || m.termHeight == 0 {
		return
	}
	if !m.layoutDirty && m.frameWidth != 0 && m.frameHeight != 0 {
		return
	}

	eventHeight := m.computeEventHeight()
	frameSpace := max(minFrameHeight, m.termHeight-eventHeight-frameGap)

	width := clamp(m.maxWidth, 20, m.termWidth-4)
	height := clamp(m.maxHeight, minFrameHeight, frameSpace)
	if m.fullscreen {
		width = clamp(m.termWidth, 20, m.termWidth)
		height = clamp(frameSpace, minFrameHeight, frameSpace)
	}

	innerWidth := max(1, width-2)

	m.frameWidth = width
	m.frameHeight = height
	m.innerWidth = innerWidth
	m.innerHeight = max(1, height-2)
	m.eventHeight = eventHeight
	m.layoutDirty = false

	if m.events != nil && eventHeight > 0 {
		m.events.SetSize(m.termWidth, eventHeight)
	}
}

func (m *testbedModel) computeEventHeight() int {
	if m.events == nil {
		return 0
	}
	maxAvailable := m.termHeight - minFrameHeight - frameGap
	if maxAvailable < minEventHeight {
		return 0
	}
	desired := clamp(m.termHeight/4, minEventHeight, maxEventHeight)
	if desired > maxAvailable {
		desired = maxAvailable
	}
	return desired
}

func (m *testbedModel) recordEvent(msg tea.Msg) {
	if m.events == nil {
		return
	}
	source := "tea"
	if s, ok := eventSource(msg); ok {
		source = s
	}
	detail := describeMsg(msg)
	entry := eventviewer.Entry{
		Timestamp: time.Now(),
		Source:    source,
		Summary:   fmt.Sprintf("%T", msg),
		Detail:    detail,
		Level:     eventviewer.LevelInfo,
	}
	m.events.Append(entry)
}

func describeMsg(msg tea.Msg) string {
	if d, ok := msg.(interface{ Describe() string }); ok {
		return d.Describe()
	}
	switch v := msg.(type) {
	case tea.KeyMsg:
		return fmt.Sprintf("key=%q", v.String())
	case tea.WindowSizeMsg:
		return fmt.Sprintf("size=%dx%d", v.Width, v.Height)
	case tea.MouseMsg:
		return fmt.Sprintf("mouse=%s", v)
	default:
		return ""
	}
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func offsetCursor(cursor *tea.Cursor, dx, dy int) *tea.Cursor {
	if cursor == nil {
		return nil
	}
	clone := *cursor
	clone.Position.X += dx
	clone.Position.Y += dy
	return &clone
}

func eventSource(msg tea.Msg) (string, bool) {
	switch v := msg.(type) {
	case events.CollectionHighlightMsg:
		return string(v.Component), true
	case events.CollectionSelectMsg:
		return string(v.Component), true
	case events.BulletHighlightMsg:
		return string(v.Component), true
	case events.BulletSelectMsg:
		return string(v.Component), true
	case events.CollectionChangeMsg:
		return string(v.Component), true
	case events.BulletChangeMsg:
		return string(v.Component), true
	case events.CommandChangeMsg:
		return string(v.Component), true
	case events.CommandSubmitMsg:
		return string(v.Component), true
	case events.CommandCancelMsg:
		return string(v.Component), true
	case events.FocusMsg:
		return string(v.Component), true
	case events.BlurMsg:
		return string(v.Component), true
	default:
		return "", false
	}
}

const (
	minFrameHeight = 12
	minEventHeight = 5
	maxEventHeight = 12
	frameGap       = 1
)
