package newapp

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/store"
	"tableflip.dev/bujo/pkg/timeutil"
	"tableflip.dev/bujo/pkg/tui/components/command"
	"tableflip.dev/bujo/pkg/tui/components/eventviewer"
	"tableflip.dev/bujo/pkg/tui/events"
)

type reportLoadedMsg struct {
	result app.ReportResult
	err    error
}

type reportClosedMsg struct{}

// Model composes the new TUI surface. It currently mounts the command
// component and, when requested, an event viewer docked to the bottom of the
// main content area.
type Model struct {
	service *app.Service

	width  int
	height int

	command *command.Model

	debugEnabled bool
	eventViewer  *eventviewer.Model

	cachePath     string
	dataSource    string
	report        *reportOverlay
	reportVisible bool
}

// New constructs a root model with the provided service.
func New(service *app.Service) *Model {
	cachePath := os.Getenv("BUJO_CACHE_PATH")
	if cachePath == "" {
		cachePath = "(BUJO_CACHE_PATH not set)"
	}
	dataSource := os.Getenv("BUJO_PATH")
	if dataSource == "" {
		if cfg, err := store.LoadConfig(); err == nil && cfg != nil {
			dataSource = cfg.BasePath()
		} else {
			dataSource = "(BUJO_PATH not set)"
		}
	}

	cmd := command.NewModel(command.Options{
		ID:           events.ComponentID("root-command"),
		PromptPrefix: ":",
		StatusText:   "Ready",
	})
	cmd.SetSuggestions([]command.SuggestionOption{
		{Name: "help", Description: "Show command tips"},
		{Name: "quit", Description: "Exit bujo"},
		{Name: "report", Description: "Show completed entries report"},
		{Name: "debug", Description: "Toggle debug event viewer"},
	})
	return &Model{
		service:    service,
		command:    cmd,
		cachePath:  cachePath,
		dataSource: dataSource,
	}
}

// Run launches the Bubble Tea program that renders the new UI.
func Run(service *app.Service) error {
	p := tea.NewProgram(New(service), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	if m.command != nil {
		return m.command.Init()
	}
	return nil
}

// Update routes Bubble Tea messages to composed components.

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.noteEvent(msg)

	var cmds []tea.Cmd

	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = v.Width
		m.height = v.Height
		m.layoutContent()
	case tea.KeyMsg:
		if v.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case events.CommandSubmitMsg:
		if m.command != nil && v.Component == m.command.ID() {
			raw := strings.TrimSpace(v.Value)
			if raw == "" {
				m.command.SetStatus("Commands: :quit, :debug, :report [window], :help")
				break
			}
			parts := strings.Fields(raw)
			if len(parts) == 0 {
				m.command.SetStatus("Commands: :quit, :debug, :report [window], :help")
				break
			}
			cmdName := strings.ToLower(parts[0])
			arg := ""
			if len(parts) > 1 {
				arg = strings.Join(parts[1:], " ")
			}
			switch cmdName {
			case "quit", "exit", "q":
				return m, tea.Quit
			case "help":
				m.command.SetStatus("Commands: :quit, :debug, :report [window], :help")
			case "debug":
				m.toggleDebug()
			case "report":
				cmd, state := m.showReportOverlay(arg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
				switch state {
				case "opened":
					m.command.SetStatus("Report overlay opened")
				case "closed":
					m.command.SetStatus("Report overlay closed")
				case "error":
					// status set inside showReportOverlay
				}
				m.layoutContent()
			default:
				m.command.SetStatus("Unhandled command: " + cmdName)
			}
		}
	case events.CommandCancelMsg:
		if m.command != nil && v.Component == m.command.ID() {
			m.command.SetStatus("Ready")
		}
	case reportClosedMsg:
		m.reportVisible = false
		m.report = nil
		if m.command != nil {
			m.command.SetStatus("Report overlay closed")
		}
		m.layoutContent()
	}

	if m.command != nil {
		next, cmd := m.command.Update(msg)
		if cm, ok := next.(*command.Model); ok {
			m.command = cm
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

// View renders the composed UI.
func (m *Model) View() (string, *tea.Cursor) {
	if m.command == nil {
		return "initializing…", nil
	}
	return m.command.View()
}

func (m *Model) layoutContent() {
	if m.command == nil {
		return
	}
	if m.width <= 0 {
		m.width = 1
	}
	if m.height <= 0 {
		m.height = 1
	}

	m.command.SetSize(m.width, m.height)

	totalRows := maxInt(1, m.height-1)
	debugRows := 0
	if m.debugEnabled {
		if m.eventViewer == nil {
			m.eventViewer = eventviewer.NewModel(400)
		}
		debugRows = m.computeDebugHeight(totalRows)
		if debugRows > 0 {
			m.eventViewer.SetSize(m.width, debugRows)
		}
	} else {
		m.eventViewer = nil
	}

	m.command.SetContent(m.composeBody(totalRows, debugRows), nil)
}

func (m *Model) composeBody(totalRows, debugRows int) string {
	if totalRows <= 0 {
		return ""
	}

	mainRows := totalRows
	if debugRows > 0 && debugRows < totalRows {
		mainRows = totalRows - debugRows
		if mainRows < 1 {
			mainRows = 1
			debugRows = totalRows - mainRows
			if debugRows < 0 {
				debugRows = 0
			}
		}
	}

	lines := make([]string, 0, mainRows)
	lines = append(lines, m.clipLine("New UI scaffold"))
	lines = append(lines, m.clipLine(fmt.Sprintf("Data source: %s", m.dataSource)))
	lines = append(lines, m.clipLine(fmt.Sprintf("Cache path: %s", m.cachePath)))
	debugState := "off"
	if m.debugEnabled {
		debugState = "on"
	}
	lines = append(lines, m.clipLine(fmt.Sprintf("Debug window: %s (:debug)", debugState)))
	reportState := "hidden"
	if m.reportVisible {
		reportState = "visible"
	}
	lines = append(lines, m.clipLine(fmt.Sprintf("Report overlay: %s (:report)", reportState)))
	if len(lines) > mainRows {
		lines = lines[:mainRows]
	}
	for len(lines) < mainRows {
		lines = append(lines, "")
	}
	mainView := strings.Join(lines, "\n")

	if debugRows > 0 && m.eventViewer != nil {
		debugView := m.eventViewer.View()
		if mainView == "" {
			return debugView
		}
		return mainView + "\n" + debugView
	}

	return mainView
}

func (m *Model) clipLine(text string) string {
	if m.width <= 0 {
		return text
	}
	if len(text) <= m.width {
		return text
	}
	if m.width <= 3 {
		return text[:m.width]
	}
	return text[:m.width-3] + "..."
}

func (m *Model) toggleDebug() {
	if m.debugEnabled {
		m.debugEnabled = false
		m.eventViewer = nil
		if m.command != nil {
			m.command.SetStatus("Debug log hidden")
		}
		m.layoutContent()
		return
	}

	m.debugEnabled = true
	if m.eventViewer == nil {
		m.eventViewer = eventviewer.NewModel(400)
	}
	m.appendEvent(eventviewer.Entry{
		Summary: "debug",
		Detail:  "Debug window enabled",
		Source:  "ui",
	})
	if m.command != nil {
		m.command.SetStatus("Debug log visible")
	}
	m.layoutContent()
}

func (m *Model) noteEvent(msg tea.Msg) {
	if m.eventViewer == nil {
		return
	}

	source := "tea"
	if s, ok := eventSource(msg); ok && s != "" {
		source = s
	}

	entry := eventviewer.Entry{
		Timestamp: time.Now(),
		Source:    source,
		Summary:   fmt.Sprintf("%T", msg),
		Detail:    describeMsg(msg),
		Level:     eventviewer.LevelInfo,
	}
	if entry.Detail == "" {
		entry.Detail = fmt.Sprintf("%v", msg)
	}
	m.eventViewer.Append(entry)
	m.layoutContent()
}

func (m *Model) showReportOverlay(arg string) (tea.Cmd, string) {
	if m.command == nil {
		return nil, "noop"
	}
	if m.service == nil {
		m.command.SetStatus("Report unavailable: service offline")
		return nil, "error"
	}
	if m.reportVisible {
		m.command.CloseOverlay()
		m.reportVisible = false
		m.report = nil
		return nil, "closed"
	}
	dur, label, err := m.parseReportWindow(arg)
	if err != nil {
		m.command.SetStatus("Report: " + err.Error())
		return nil, "error"
	}
	overlay := newReportOverlay(m.service, dur, label)
	placement := m.reportPlacement()
	m.report = overlay
	m.reportVisible = true
	return m.command.SetOverlay(overlay, placement), "opened"
}

func (m *Model) reportPlacement() command.OverlayPlacement {
	availableWidth := m.width
	if availableWidth <= 0 {
		availableWidth = 1
	}
	width := int(math.Round(float64(availableWidth) * 0.9))
	if width <= 0 || width > availableWidth {
		width = availableWidth
	}
	if width < 20 {
		width = minInt(20, availableWidth)
	}
	availableHeight := m.height - 1
	if availableHeight <= 0 {
		availableHeight = 1
	}
	height := int(math.Round(float64(availableHeight) * 0.9))
	if height <= 0 || height > availableHeight {
		height = availableHeight
	}
	if height < 5 {
		height = minInt(availableHeight, 5)
	}
	return command.OverlayPlacement{
		Width:      width,
		Height:     height,
		Horizontal: lipgloss.Center,
		Vertical:   lipgloss.Center,
	}
}

func (m *Model) parseReportWindow(spec string) (time.Duration, string, error) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return timeutil.ParseWindow(timeutil.DefaultWindow)
	}
	normalized := strings.ReplaceAll(trimmed, " ", "")
	return timeutil.ParseWindow(normalized)
}

func (m *Model) appendEvent(entry eventviewer.Entry) {
	if m.eventViewer == nil {
		return
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if entry.Source == "" {
		entry.Source = "ui"
	}
	if entry.Summary == "" {
		entry.Summary = "event"
	}
	m.eventViewer.Append(entry)
}

func (m *Model) computeDebugHeight(totalRows int) int {
	if totalRows <= 4 {
		return 0
	}
	minHeight := 5
	maxHeight := totalRows - 1
	if maxHeight < minHeight {
		return maxHeight
	}
	desired := clamp(totalRows/3, minHeight, minInt(12, maxHeight))
	return desired
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

func clamp(value, lower, upper int) int {
	if upper <= 0 {
		return lower
	}
	if value < lower {
		return lower
	}
	if value > upper {
		return upper
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
