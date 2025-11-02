package newapp

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/davecgh/go-spew/spew"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/store"
	"tableflip.dev/bujo/pkg/timeutil"
	cachepkg "tableflip.dev/bujo/pkg/tui/cache"
	collectiondetail "tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	collectionnav "tableflip.dev/bujo/pkg/tui/components/collectionnav"
	"tableflip.dev/bujo/pkg/tui/components/command"
	dummyview "tableflip.dev/bujo/pkg/tui/components/dummy"
	"tableflip.dev/bujo/pkg/tui/components/eventviewer"
	helpview "tableflip.dev/bujo/pkg/tui/components/help"
	journalcomponent "tableflip.dev/bujo/pkg/tui/components/journal"
	overlaypane "tableflip.dev/bujo/pkg/tui/components/overlaypane"
	"tableflip.dev/bujo/pkg/tui/events"
)

type reportLoadedMsg struct {
	result app.ReportResult
	err    error
}

type reportClosedMsg struct{}

type journalLoadedMsg struct {
	snapshot journalSnapshot
	err      error
}

// Model composes the new TUI surface. It currently mounts the command
// component and, when requested, an event viewer docked to the bottom of the
// main content area.
type Model struct {
	service *app.Service

	width  int
	height int

	command     *command.Model
	overlayPane *overlaypane.Model

	debugEnabled bool
	eventViewer  *eventviewer.Model

	cachePath     string
	dataSource    string
	report        *reportOverlay
	reportVisible bool

	dump io.Writer

	helpVisible  bool
	helpReturn   journalcomponent.FocusPane
	helpHadFocus bool

	commandActive bool
	commandReturn journalcomponent.FocusPane

	journalNav     *collectionnav.Model
	journalDetail  *collectiondetail.Model
	journalCache   *cachepkg.Cache
	journalView    *journalcomponent.Model
	loadingJournal bool
	journalError   error

	focusStack []focusTarget
}

type focusKind int

const (
	focusKindUnknown focusKind = iota
	focusKindJournalNav
	focusKindJournalDetail
	focusKindCommand
	focusKindOverlay
)

type overlayKind int

const (
	overlayKindNone overlayKind = iota
	overlayKindHelp
	overlayKindReport
)

type focusTarget struct {
	kind    focusKind
	pane    journalcomponent.FocusPane
	overlay overlayKind
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
		service:     service,
		command:     cmd,
		overlayPane: overlaypane.New(1, 1),
		cachePath:   cachePath,
		dataSource:  dataSource,
	}
}

// Run launches the Bubble Tea program that renders the new UI.
func Run(service *app.Service) error {
	var dumpFile *os.File
	if _, ok := os.LookupEnv("DEBUG"); ok {
		f, err := os.OpenFile("messages.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("open message dump: %w", err)
		}
		dumpFile = f
	}
	model := New(service)
	if dumpFile != nil {
		model.dump = dumpFile
		model.logf("data source: %s cache path: %s", model.dataSource, model.cachePath)
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	if dumpFile != nil {
		_ = dumpFile.Close()
	}
	return err
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.command != nil {
		if cmd := m.command.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if cmd := m.loadJournalSnapshot(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// Update routes Bubble Tea messages to composed components.

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.noteEvent(msg)

	var cmds []tea.Cmd
	skipCommandUpdate := false
	skipJournalKey := false

	if m.dump != nil {
		fmt.Fprintf(m.dump, "%s ", time.Now().Format("2006-01-02T15:04:05"))
		spew.Fdump(m.dump, msg)
	}

	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = v.Width
		m.height = v.Height
		m.layoutContent()
	case tea.KeyMsg:
		switch v.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.overlayPane != nil && m.overlayPane.HasOverlay() {
				if cmd := m.dismissActiveOverlay(); cmd != nil {
					cmds = append(cmds, cmd)
				}
				skipCommandUpdate = true
				skipJournalKey = true
			}
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
				cmd, state := m.toggleHelpOverlay()
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
				switch state {
				case "opened":
					m.command.SetStatus("Help overlay opened (Esc or : to close)")
				case "closed":
					m.command.SetStatus("Help overlay closed")
				case "noop":
					m.command.SetStatus("Help unavailable")
				}
				m.layoutContent()
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
			_ = m.dropFocusKind(focusKindCommand)
		}
	case events.CommandCancelMsg:
		if m.command != nil && v.Component == m.command.ID() {
			m.command.SetStatus("Ready")
			if m.commandActive {
				m.commandActive = false
				if !m.helpVisible {
					if cmd := m.focusJournalPane(m.commandReturn); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			}
			_ = m.dropFocusKind(focusKindCommand)
		}
	case events.CommandChangeMsg:
		if m.command != nil && v.Component == m.command.ID() {
			if v.Mode == events.CommandModeInput {
				if !m.commandActive {
					if m.journalView != nil {
						m.commandReturn = m.journalView.FocusedPane()
					} else {
						m.commandReturn = journalcomponent.FocusNav
					}
					m.commandActive = true
					cmds = append(cmds, m.blurJournalPanes()...)
					m.pushFocus(focusTarget{kind: focusKindCommand})
				}
			} else {
				if m.commandActive {
					m.commandActive = false
					if !m.helpVisible {
						if cmd := m.focusJournalPane(m.commandReturn); cmd != nil {
							cmds = append(cmds, cmd)
						}
					}
				}
				_ = m.dropFocusKind(focusKindCommand)
			}
		} else {
			_ = m.dropFocusKind(focusKindCommand)
		}
		m.layoutContent()
	case reportClosedMsg:
		m.reportVisible = false
		m.report = nil
		if m.command != nil {
			m.command.SetStatus("Report overlay closed")
		}
		_, _ = m.popFocusKind(focusKindOverlay)
		if cmd := m.restoreFocusAfterOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.layoutContent()
	case journalLoadedMsg:
		m.loadingJournal = false
		if v.err != nil {
			m.journalError = v.err
			if m.command != nil {
				m.command.SetStatus("Journal load failed: " + v.err.Error())
			}
			break
		}
		snap := v.snapshot
		cache := cachepkg.New(events.ComponentID("journal-cache"))
		cache.SetCollections(snap.metas)
		cache.SetSections(snap.sections)
		nav := collectionnav.NewModel(snap.parsed)
		nav.SetID(events.ComponentID("MainNav"))
		detail := collectiondetail.NewModel(snap.sections)
		detail.SetID(events.ComponentID("DetailPane"))
		detail.SetSourceNav(nav.ID())
		if m.dump != nil {
			detail.SetDebugWriter(m.dump)
		}
		journal := journalcomponent.NewModel(nav, detail, cache)
		if cmd := journal.FocusNav(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if m.command != nil {
			m.command.Blur()
		}
		m.journalCache = cache
		m.journalNav = nav
		m.journalDetail = detail
		m.journalView = journal
		m.journalError = nil
		if m.command != nil {
			m.command.SetStatus("Journal loaded")
		}
		m.layoutContent()
	case events.FocusMsg:
		m.handleFocusMsg(v)
	case events.BlurMsg:
		m.handleBlurMsg(v)
	}

	if m.command != nil && !skipCommandUpdate {
		next, cmd := m.command.Update(msg)
		if cm, ok := next.(*command.Model); ok {
			m.command = cm
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if m.overlayPane != nil {
		if cmd := m.overlayPane.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if !m.overlayPane.HasOverlay() {
			if m.helpVisible {
				if cmd := m.closeHelpOverlay(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else if m.reportVisible {
				if cmd := m.closeReportOverlay(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
	}

	if m.journalView != nil {
		skipKey := skipJournalKey
		if !skipKey && m.helpVisible {
			if _, isKey := msg.(tea.KeyMsg); isKey {
				skipKey = true
			}
		}
		if !skipKey {
			next, cmd := m.journalView.Update(msg)
			if jm, ok := next.(*journalcomponent.Model); ok {
				m.journalView = jm
			}
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	m.layoutContent()

	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) logf(format string, args ...interface{}) {
	if m.dump == nil {
		return
	}
	fmt.Fprintf(m.dump, "%s %s\n", time.Now().Format("2006-01-02T15:04:05"), fmt.Sprintf(format, args...))
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

	mainRows := totalRows
	if debugRows > 0 && debugRows < totalRows {
		mainRows = totalRows - debugRows
	}
	if mainRows < 1 {
		mainRows = 1
	}
	m.logf("layout height=%d total=%d debug=%d main=%d", m.height, totalRows, debugRows, mainRows)
	mainView, mainCursor := m.mainContent(mainRows)
	if m.overlayPane == nil {
		m.overlayPane = overlaypane.New(m.width, mainRows)
	}
	m.overlayPane.SetSize(m.width, mainRows)
	m.overlayPane.SetBackground(mainView, mainCursor)
	composed, composedCursor := m.overlayPane.View()
	body := composed
	if debugRows > 0 && m.eventViewer != nil {
		debugView := m.eventViewer.View()
		if body != "" {
			body = body + "\n" + debugView
		} else {
			body = debugView
		}
	}
	m.command.SetContent(body, composedCursor)
}

func (m *Model) mainContent(height int) (string, *tea.Cursor) {
	if m.journalView != nil {
		if height < 1 {
			height = 1
		}
		m.journalView.SetSize(m.width, height)
		m.logf("journal.SetSize width=%d height=%d", m.width, height)
		view, cursor := m.journalView.View()
		viewLines := strings.Split(view, "\n")
		if len(viewLines) > 0 && viewLines[len(viewLines)-1] == "" {
			viewLines = viewLines[:len(viewLines)-1]
		}
		if len(viewLines) > height {
			viewLines = viewLines[:height]
		}
		for len(viewLines) < height {
			viewLines = append(viewLines, "")
		}
		m.logf("journal view lines=%d height=%d", len(viewLines), height)
		if cursor != nil {
			m.logf("journal cursor x=%d y=%d", cursor.X, cursor.Y)
		}
		body := strings.Join(viewLines, "\n")
		return body, cursor
	}

	var lines []string
	if m.loadingJournal {
		lines = append(lines, m.clipLine("Loading journal…"))
	} else if m.journalError != nil {
		lines = append(lines, m.clipLine("Journal load failed: "+m.journalError.Error()))
	} else {
		lines = append(lines, m.clipLine("Journal not available"))
	}
	return strings.Join(lines, "\n"), nil
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
	if m.overlayPane == nil {
		m.overlayPane = overlaypane.New(m.width, maxInt(1, m.height-1))
	}
	if m.service == nil {
		m.command.SetStatus("Report unavailable: service offline")
		return nil, "error"
	}
	if m.reportVisible {
		return m.closeReportOverlay(), "closed"
	}
	dur, label, err := m.parseReportWindow(arg)
	if err != nil {
		m.command.SetStatus("Report: " + err.Error())
		return nil, "error"
	}
	var cmds []tea.Cmd
	if m.helpVisible {
		if cmd := m.closeHelpOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	overlay := newReportOverlay(m.service, dur, label)
	placement := m.reportPlacement()
	width := placement.Width
	if width <= 0 {
		width = m.width
	}
	height := placement.Height
	if height <= 0 {
		height = maxInt(10, m.height-1)
	}
	m.report = overlay
	overlay.SetSize(width, height)
	cmd := m.overlayPane.SetOverlay(overlay, placement)
	m.reportVisible = true
	_ = m.dropFocusKind(focusKindCommand)
	m.pushFocus(focusTarget{kind: focusKindOverlay, overlay: overlayKindReport})
	cmds = append(cmds, m.blurJournalPanes()...)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	if focusCmd := m.overlayPane.Focus(); focusCmd != nil {
		cmds = append(cmds, focusCmd)
	}
	if len(cmds) == 0 {
		return nil, "opened"
	}
	return tea.Batch(cmds...), "opened"
}

func (m *Model) loadJournalSnapshot() tea.Cmd {
	if m.service == nil {
		m.journalError = fmt.Errorf("service unavailable")
		return nil
	}
	m.loadingJournal = true
	svc := m.service
	return func() tea.Msg {
		snapshot, err := buildJournalSnapshot(context.Background(), svc)
		return journalLoadedMsg{snapshot: snapshot, err: err}
	}
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
		Vertical:   lipgloss.Top,
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

func (m *Model) toggleHelpOverlay() (tea.Cmd, string) {
	if m.command == nil {
		return nil, "noop"
	}
	if m.overlayPane == nil {
		m.overlayPane = overlaypane.New(m.width, maxInt(1, m.height-1))
	}
	if m.helpVisible {
		return m.closeHelpOverlay(), "closed"
	}
	cmd := m.openHelpOverlay()
	if cmd == nil {
		return nil, "opened"
	}
	return cmd, "opened"
}

func (m *Model) helpPlacement() command.OverlayPlacement {
	availableWidth := m.width
	if availableWidth <= 0 {
		availableWidth = 1
	}
	width := int(math.Round(float64(availableWidth) * 0.75))
	if width <= 0 || width > availableWidth {
		width = availableWidth
	}
	if width < 38 {
		width = minInt(availableWidth, 38)
	}
	availableHeight := m.height - 1
	if availableHeight <= 0 {
		availableHeight = 1
	}
	height := int(math.Round(float64(availableHeight) * 0.8))
	if height <= 0 || height > availableHeight {
		height = availableHeight
	}
	if height < 10 {
		height = minInt(availableHeight, 10)
	}
	return command.OverlayPlacement{
		Width:      width,
		Height:     height,
		Horizontal: lipgloss.Center,
		Vertical:   lipgloss.Top,
	}
}

func (m *Model) openHelpOverlay() tea.Cmd {
	if m.overlayPane == nil {
		m.overlayPane = overlaypane.New(m.width, maxInt(1, m.height-1))
	}
	placement := m.helpPlacement()
	width := placement.Width
	if width <= 0 {
		width = m.width
	}
	height := placement.Height
	if height <= 0 {
		height = maxInt(10, m.height-1)
	}
	var overlay command.Overlay
	if os.Getenv("BUJO_HELP_DUMMY") == "1" {
		overlay = dummyview.New(width, height)
	} else {
		overlay = helpview.New(width, height)
	}
	overlay.SetSize(width, height)

	var cmds []tea.Cmd
	if m.reportVisible {
		if cmd := m.closeReportOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.journalView != nil {
		m.helpReturn = m.journalView.FocusedPane()
	} else {
		m.helpReturn = journalcomponent.FocusNav
	}
	m.helpHadFocus = true
	cmds = append(cmds, m.blurJournalPanes()...)
	if cmd := m.overlayPane.SetOverlay(overlay, placement); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if focusCmd := m.overlayPane.Focus(); focusCmd != nil {
		cmds = append(cmds, focusCmd)
	}
	m.helpVisible = true
	_ = m.dropFocusKind(focusKindCommand)
	m.pushFocus(focusTarget{kind: focusKindOverlay, overlay: overlayKindHelp})
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) closeHelpOverlay() tea.Cmd {
	if !m.helpVisible {
		return nil
	}
	var cmds []tea.Cmd
	if m.overlayPane != nil {
		if cmd := m.overlayPane.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.overlayPane.ClearOverlay()
	}
	m.helpVisible = false
	m.helpHadFocus = false
	_, _ = m.popFocusKind(focusKindOverlay)
	if cmd := m.restoreFocusAfterOverlay(); cmd != nil {
		cmds = append(cmds, cmd)
	} else if m.journalView != nil {
		var restore tea.Cmd
		switch m.helpReturn {
		case journalcomponent.FocusDetail:
			restore = m.journalView.FocusDetail()
		default:
			restore = m.journalView.FocusNav()
		}
		if restore != nil {
			cmds = append(cmds, restore)
		}
	}
	if m.command != nil {
		m.command.SetStatus("Help overlay closed")
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) closeReportOverlay() tea.Cmd {
	if !m.reportVisible {
		return nil
	}
	var cmds []tea.Cmd
	if m.overlayPane != nil {
		if cmd := m.overlayPane.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.overlayPane.ClearOverlay()
	}
	m.reportVisible = false
	m.report = nil
	_, _ = m.popFocusKind(focusKindOverlay)
	if cmd := m.restoreFocusAfterOverlay(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) dismissActiveOverlay() tea.Cmd {
	if m.overlayPane == nil || !m.overlayPane.HasOverlay() {
		return nil
	}
	if m.helpVisible {
		return m.closeHelpOverlay()
	}
	if m.reportVisible {
		cmd := m.closeReportOverlay()
		if m.command != nil {
			m.command.SetStatus("Report overlay closed")
		}
		return cmd
	}
	m.overlayPane.ClearOverlay()
	_, _ = m.popFocusKind(focusKindOverlay)
	return m.restoreFocusAfterOverlay()
}

func (m *Model) handleFocusMsg(msg events.FocusMsg) {
	if m.journalNav != nil && msg.Component == m.journalNav.ID() {
		m.pushFocus(focusTarget{kind: focusKindJournalNav, pane: journalcomponent.FocusNav})
		return
	}
	if m.journalDetail != nil && msg.Component == m.journalDetail.ID() {
		m.pushFocus(focusTarget{kind: focusKindJournalDetail, pane: journalcomponent.FocusDetail})
	}
}

func (m *Model) handleBlurMsg(msg events.BlurMsg) {
	_ = msg
	// Retain focus history on blur so overlays can restore prior panes.
}

func (m *Model) restoreFocusAfterOverlay() tea.Cmd {
	var cmds []tea.Cmd
	for {
		target, ok := m.topFocus()
		if !ok {
			break
		}
		if target.kind == focusKindCommand {
			if !m.commandActive {
				_ = m.dropFocusKind(focusKindCommand)
				continue
			}
			break
		}
		if cmd := m.applyFocusTarget(target); cmd != nil {
			cmds = append(cmds, cmd)
		}
		break
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) pushFocus(target focusTarget) {
	if target.kind == focusKindUnknown {
		return
	}
	for i := len(m.focusStack) - 1; i >= 0; i-- {
		if m.focusStack[i].kind == target.kind {
			m.focusStack = m.focusStack[:i]
			break
		}
	}
	m.focusStack = append(m.focusStack, target)
}

func (m *Model) dropFocusKind(kind focusKind) bool {
	for i := len(m.focusStack) - 1; i >= 0; i-- {
		if m.focusStack[i].kind == kind {
			m.focusStack = append(m.focusStack[:i], m.focusStack[i+1:]...)
			return true
		}
	}
	return false
}

func (m *Model) popFocusKind(kind focusKind) (focusTarget, bool) {
	for i := len(m.focusStack) - 1; i >= 0; i-- {
		if m.focusStack[i].kind == kind {
			target := m.focusStack[i]
			m.focusStack = append(m.focusStack[:i], m.focusStack[i+1:]...)
			return target, true
		}
	}
	return focusTarget{}, false
}

func (m *Model) topFocus() (focusTarget, bool) {
	if len(m.focusStack) == 0 {
		return focusTarget{}, false
	}
	return m.focusStack[len(m.focusStack)-1], true
}

func (m *Model) applyFocusTarget(target focusTarget) tea.Cmd {
	switch target.kind {
	case focusKindJournalNav:
		if m.journalView != nil {
			return m.journalView.FocusNav()
		}
	case focusKindJournalDetail:
		if m.journalView != nil {
			return m.journalView.FocusDetail()
		}
	case focusKindCommand:
		if m.command != nil {
			m.command.Focus()
		}
	case focusKindOverlay:
		if m.overlayPane != nil && m.overlayPane.HasOverlay() {
			return m.overlayPane.Focus()
		}
	}
	return nil
}

func (m *Model) blurJournalPanes() []tea.Cmd {
	var cmds []tea.Cmd
	if m.journalNav != nil {
		if cmd := m.journalNav.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.journalDetail != nil {
		if cmd := m.journalDetail.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}

func (m *Model) focusJournalPane(pane journalcomponent.FocusPane) tea.Cmd {
	if m.journalView == nil {
		return nil
	}
	switch pane {
	case journalcomponent.FocusDetail:
		return m.journalView.FocusDetail()
	default:
		return m.journalView.FocusNav()
	}
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
