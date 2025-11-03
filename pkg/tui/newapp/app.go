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
	"tableflip.dev/bujo/pkg/collection"
	viewmodel "tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/store"
	"tableflip.dev/bujo/pkg/timeutil"
	cachepkg "tableflip.dev/bujo/pkg/tui/cache"
	"tableflip.dev/bujo/pkg/tui/components/addtask"
	bulletdetail "tableflip.dev/bujo/pkg/tui/components/bulletdetail"
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

type cacheMsg struct {
	payload tea.Msg
}

type dayCheckMsg struct{}

type watchStartedMsg struct {
	ch     <-chan store.Event
	cancel context.CancelFunc
	err    error
}

type watchEventMsg struct {
	event store.Event
}

type watchStoppedMsg struct{}

type watchErrorMsg struct {
	err error
}

type journalLoadedMsg struct {
	snapshot cachepkg.Snapshot
	err      error
}

type bulletDetailLoadedMsg struct {
	requestID string
	entry     *entry.Entry
	err       error
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

	helpVisible      bool
	helpReturn       journalcomponent.FocusPane
	helpHadFocus     bool
	addVisible       bool
	addOverlay       *addtaskOverlay
	detailVisible    bool
	detailOverlay    *bulletdetailOverlay
	detailLoadID     string
	moveVisible      bool
	moveOverlay      *movebulletOverlay
	moveLoadID       string
	moveBulletID     string
	moveCollectionID string
	moveFutureOnly   bool

	commandActive bool
	commandReturn journalcomponent.FocusPane

	journalNav     *collectionnav.Model
	journalDetail  *collectiondetail.Model
	journalCache   *cachepkg.Cache
	journalView    *journalcomponent.Model
	loadingJournal bool
	journalError   error

	focusStack []focusTarget

	ctx    context.Context
	cancel context.CancelFunc

	watchCh     <-chan store.Event
	watchCancel context.CancelFunc

	today time.Time
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
	overlayKindAdd
	overlayKindBulletDetail
	overlayKindMove
)

type moveOverlayConfig struct {
	detail       *bulletdetail.Model
	nav          *collectionnav.Model
	bulletID     string
	collectionID string
	label        string
	status       string
	initialRef   events.CollectionRef
	futureOnly   bool
	navOnRight   bool
}

const (
	addTaskOverlayID      = events.ComponentID("addtask-overlay")
	bulletDetailOverlayID = events.ComponentID("bulletdetail-overlay")
	moveNavID             = events.ComponentID("MoveNav")
)

const dayCheckInterval = time.Minute

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
	ctx, cancel := context.WithCancel(context.Background())
	return &Model{
		service:     service,
		command:     cmd,
		overlayPane: overlaypane.New(1, 1),
		cachePath:   cachePath,
		dataSource:  dataSource,
		ctx:         ctx,
		cancel:      cancel,
		today:       startOfDay(time.Now()),
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
	defer func() {
		if model.cancel != nil {
			model.cancel()
			model.cancel = nil
		}
	}()
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithReportFocus())
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
	if cmd := m.scheduleDayCheck(); cmd != nil {
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
	case tea.FocusMsg:
		m.refreshToday(time.Now())
	case tea.BlurMsg:
		// no-op (we'll rely on periodic checks)
	case dayCheckMsg:
		m.refreshToday(time.Now())
		if cmd := m.scheduleDayCheck(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case cacheMsg:
		if cmd := cacheListenCmd(m.journalCache); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if v.payload != nil {
			nextModel, innerCmd := m.Update(v.payload)
			if innerCmd != nil {
				cmds = append(cmds, innerCmd)
			}
			if len(cmds) == 0 {
				return nextModel, nil
			}
			return nextModel, tea.Batch(cmds...)
		}
	case watchStartedMsg:
		if v.err != nil {
			if m.command != nil {
				m.command.SetStatus("Watch start failed: " + v.err.Error())
			}
			break
		}
		m.watchCh = v.ch
		m.watchCancel = v.cancel
		if cmd := m.waitForWatch(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case watchEventMsg:
		if cmd := m.handleWatchEvent(v.event); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if cmd := m.waitForWatch(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case watchStoppedMsg:
		m.stopWatch()
		if m.ctx != nil && m.service != nil {
			if cmd := startWatchCmd(m.ctx, m.service); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	case watchErrorMsg:
		if v.err != nil {
			if m.command != nil {
				m.command.SetStatus("Watch error: " + v.err.Error())
			}
			m.appendEvent(eventviewer.Entry{
				Summary: "watch",
				Detail:  v.err.Error(),
				Level:   eventviewer.LevelError,
			})
		}
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
				if m.addVisible {
					skipJournalKey = true
					break
				}
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
	case tea.QuitMsg:
		m.stopWatch()
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
	case events.AddTaskRequestMsg:
		if cmd := m.handleAddTaskRequest(v); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case events.BulletDetailRequestMsg:
		if cmd := m.handleBulletDetailRequest(v); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case events.MoveBulletRequestMsg:
		if cmd := m.handleMoveBulletRequest(v); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case events.BulletCompleteMsg:
		if cmd := m.handleBulletComplete(v); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case events.BulletStrikeMsg:
		if cmd := m.handleBulletStrike(v); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case events.BulletMoveFutureMsg:
		if cmd := m.handleBulletMoveFuture(v); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case events.BulletSignifierMsg:
		if cmd := m.handleBulletSignifier(v); cmd != nil {
			cmds = append(cmds, cmd)
		}
		skipJournalKey = true
		skipCommandUpdate = true
	case bulletDetailLoadedMsg:
		if cmd := m.handleBulletDetailLoaded(v); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case events.CollectionSelectMsg:
		if m.moveVisible && v.Component == moveNavID {
			if cmd := m.handleMoveSelection(v); cmd != nil {
				cmds = append(cmds, cmd)
			}
			skipCommandUpdate = true
			skipJournalKey = true
		}
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
		cache := cachepkg.NewWithOptions(cachepkg.Options{
			Component: events.ComponentID("journal-cache"),
			Service:   m.service,
		})
		cache.SetCollections(snap.Metas)
		cache.SetSections(snap.Sections)
		nav := collectionnav.NewModel(snap.Collections)
		if !m.today.IsZero() {
			nav.SetNow(time.Now())
		}
		nav.SetID(events.ComponentID("MainNav"))
		detail := collectiondetail.NewModel(snap.Sections)
		detail.SetID(events.ComponentID("DetailPane"))
		detail.SetSourceNav(nav.ID())
		if m.dump != nil {
			detail.SetDebugWriter(m.dump)
		}
		journal := journalcomponent.NewModel(nav, detail, cache)
		journal.SetID(events.ComponentID("JournalPane"))
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
		if cmd := cacheListenCmd(cache); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if m.service != nil && m.ctx != nil {
			m.stopWatch()
			if cmd := startWatchCmd(m.ctx, m.service); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if m.command != nil {
			m.command.SetStatus("Journal loaded")
		}
		m.layoutContent()
	case events.FocusMsg:
		m.handleFocusMsg(v)
	case events.BlurMsg:
		if cmd := m.handleBlurMsg(v); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if m.addVisible || m.detailVisible || m.moveVisible {
		skipCommandUpdate = true
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
			} else if m.addVisible {
				if cmd := m.closeAddTaskOverlay(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
	}

	if m.journalView != nil {
		skipUpdate := false
		if _, isKey := msg.(tea.KeyMsg); isKey {
			if skipJournalKey || m.helpVisible || m.addVisible || m.detailVisible || m.moveVisible {
				skipUpdate = true
			}
		}
		if !skipUpdate {
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

func (m *Model) handleAddTaskRequest(msg events.AddTaskRequestMsg) tea.Cmd {
	if msg.CollectionID == "" {
		if m.command != nil {
			m.command.SetStatus("Add task unavailable: missing collection")
		}
		return nil
	}
	if m.journalCache == nil {
		if m.command != nil {
			m.command.SetStatus("Add task unavailable: journal cache offline")
		}
		return nil
	}
	opts := addtask.Options{
		InitialCollectionID:    msg.CollectionID,
		InitialCollectionLabel: strings.TrimSpace(msg.CollectionLabel),
		InitialParentBulletID:  strings.TrimSpace(msg.ParentBulletID),
	}
	return m.openAddTaskOverlay(opts, msg)
}

func (m *Model) handleBulletDetailRequest(msg events.BulletDetailRequestMsg) tea.Cmd {
	bulletID := strings.TrimSpace(msg.Bullet.ID)
	if bulletID == "" {
		if m.command != nil {
			m.command.SetStatus("Bullet details unavailable: missing bullet ID")
		}
		return nil
	}
	if m.service == nil {
		if m.command != nil {
			m.command.SetStatus("Bullet details unavailable: service offline")
		}
		return nil
	}
	collectionID := strings.TrimSpace(msg.Collection.ID)
	if collectionID == "" {
		collectionID = strings.TrimSpace(msg.Bullet.Note)
	}
	if collectionID == "" {
		if m.command != nil {
			m.command.SetStatus("Bullet details unavailable: missing collection context")
		}
		return nil
	}
	if m.overlayPane == nil {
		m.overlayPane = overlaypane.New(m.width, maxInt(1, m.height-1))
	}
	var cmds []tea.Cmd
	if m.helpVisible {
		if cmd := m.closeHelpOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.reportVisible {
		if cmd := m.closeReportOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.detailVisible {
		if cmd := m.closeBulletDetailOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.addVisible {
		if cmd := m.closeAddTaskOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	title := msg.Collection.Title
	if strings.TrimSpace(title) == "" {
		title = collectionID
	}
	detailModel := bulletdetail.New(title, msg.Bullet.Label, collectionID, msg.Bullet.Note)
	detailModel.SetLoading(true)
	wrapper := newBulletdetailOverlay(detailModel)
	placement := command.OverlayPlacement{Fullscreen: true}
	if cmd := m.overlayPane.SetOverlay(wrapper, placement); cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.detailOverlay = wrapper
	m.detailVisible = true
	requestID := fmt.Sprintf("%s@%d", bulletID, time.Now().UnixNano())
	m.detailLoadID = requestID
	_ = m.dropFocusKind(focusKindCommand)
	m.pushFocus(focusTarget{kind: focusKindOverlay, overlay: overlayKindBulletDetail})
	cmds = append(cmds, m.blurJournalPanes()...)
	if focusCmd := m.overlayPane.Focus(); focusCmd != nil {
		cmds = append(cmds, focusCmd)
	}
	if m.command != nil {
		statusLabel := strings.TrimSpace(msg.Bullet.Label)
		if statusLabel == "" {
			statusLabel = bulletID
		}
		m.command.SetStatus("Loading details for " + statusLabel)
	}
	if loadCmd := m.loadBulletDetail(collectionID, bulletID, requestID); loadCmd != nil {
		cmds = append(cmds, loadCmd)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) handleMoveBulletRequest(msg events.MoveBulletRequestMsg) tea.Cmd {
	bulletID := strings.TrimSpace(msg.Bullet.ID)
	if bulletID == "" {
		if m.command != nil {
			m.command.SetStatus("Move unavailable: missing bullet ID")
		}
		return nil
	}
	if m.service == nil {
		if m.command != nil {
			m.command.SetStatus("Move unavailable: service offline")
		}
		return nil
	}
	if m.journalCache == nil {
		if m.command != nil {
			m.command.SetStatus("Move unavailable: journal cache offline")
		}
		return nil
	}
	collectionID := strings.TrimSpace(msg.Collection.ID)
	if collectionID == "" {
		collectionID = strings.TrimSpace(msg.Bullet.Note)
	}
	if collectionID == "" {
		if m.command != nil {
			m.command.SetStatus("Move unavailable: missing collection context")
		}
		return nil
	}
	snapshot := m.journalCache.Snapshot()
	if len(snapshot.Collections) == 0 {
		if m.command != nil {
			m.command.SetStatus("Move unavailable: no collections")
		}
		return nil
	}
	trimmedCollections := filterMoveCollections(snapshot.Collections)
	if len(trimmedCollections) == 0 {
		if m.command != nil {
			m.command.SetStatus("Move unavailable: no target collections")
		}
		return nil
	}
	title := strings.TrimSpace(msg.Collection.Title)
	if title == "" {
		title = collectionID
	}
	detailModel := bulletdetail.New(title, msg.Bullet.Label, collectionID, msg.Bullet.Note)

	nav := collectionnav.NewModel(trimmedCollections)
	nav.SetBlurOnSelect(false)
	label := strings.TrimSpace(msg.Bullet.Label)
	if label == "" {
		label = bulletID
	}
	cfg := moveOverlayConfig{
		detail:       detailModel,
		nav:          nav,
		bulletID:     bulletID,
		collectionID: collectionID,
		label:        label,
		status:       "Choose destination for " + label,
		initialRef:   events.CollectionRef{ID: collectionID, Name: title},
		futureOnly:   false,
		navOnRight:   true,
	}
	return m.openMoveOverlay(cfg)
}

func (m *Model) openMoveOverlay(cfg moveOverlayConfig) tea.Cmd {
	if cfg.bulletID == "" {
		return nil
	}
	if m.overlayPane == nil {
		m.overlayPane = overlaypane.New(m.width, maxInt(1, m.height-1))
	}
	var cmds []tea.Cmd
	if m.helpVisible {
		if cmd := m.closeHelpOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.reportVisible {
		if cmd := m.closeReportOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.addVisible {
		if cmd := m.closeAddTaskOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.detailVisible {
		if cmd := m.closeBulletDetailOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.moveVisible {
		if cmd := m.closeMoveOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if cfg.detail != nil {
		cfg.detail.SetLoading(true)
	}
	if cfg.nav != nil {
		cfg.nav.SetID(moveNavID)
	}
	mOverlay := newMovebulletOverlay(cfg.detail, cfg.nav, cfg.navOnRight)
	placement := command.OverlayPlacement{Fullscreen: true}
	if cmd := m.overlayPane.SetOverlay(mOverlay, placement); cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.moveOverlay = mOverlay
	m.moveVisible = true
	m.moveBulletID = cfg.bulletID
	m.moveCollectionID = cfg.collectionID
	m.moveFutureOnly = cfg.futureOnly
	reqID := fmt.Sprintf("%s@%d", cfg.bulletID, time.Now().UnixNano())
	m.moveLoadID = reqID
	_ = m.dropFocusKind(focusKindCommand)
	m.pushFocus(focusTarget{kind: focusKindOverlay, overlay: overlayKindMove})
	cmds = append(cmds, m.blurJournalPanes()...)
	if focusCmd := m.overlayPane.Focus(); focusCmd != nil {
		cmds = append(cmds, focusCmd)
	}
	if cfg.nav != nil {
		if cfg.initialRef.ID != "" || cfg.initialRef.Name != "" {
			if cmd := cfg.nav.SelectCollection(cfg.initialRef); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	if m.command != nil {
		label := cfg.label
		if label == "" {
			label = cfg.bulletID
		}
		status := strings.TrimSpace(cfg.status)
		if status == "" {
			status = "Choose destination for " + label
		}
		m.command.SetStatus(status)
	}
	if loadCmd := m.loadBulletDetail(cfg.collectionID, cfg.bulletID, reqID); loadCmd != nil {
		cmds = append(cmds, loadCmd)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) futureMoveCollections(ctx context.Context, now time.Time) ([]*viewmodel.ParsedCollection, error) {
	if m.service == nil {
		return nil, fmt.Errorf("service offline")
	}
	metas, err := m.service.CollectionsMeta(ctx, "")
	if err != nil {
		return nil, err
	}
	return futureCollectionsFromMetas(metas, now), nil
}

func futureCollectionsFromMetas(metas []collection.Meta, now time.Time) []*viewmodel.ParsedCollection {
	existing := make(map[string]collection.Meta, len(metas))
	for _, meta := range metas {
		name := strings.TrimSpace(meta.Name)
		if name == "" {
			continue
		}
		if meta.Type == "" {
			meta.Type = collection.TypeGeneric
		}
		existing[name] = meta
	}
	futureType := collection.TypeMonthly
	if meta, ok := existing["Future"]; ok && meta.Type != "" {
		futureType = meta.Type
	}
	treeMetas := []collection.Meta{{Name: "Future", Type: futureType}}
	base := startOfMonth(now)
	if base.IsZero() {
		base = startOfMonth(time.Now())
	}
	for i := 1; i <= 12; i++ {
		monthTime := base.AddDate(0, i, 0)
		monthName := monthTime.Format("January 2006")
		full := fmt.Sprintf("Future/%s", monthName)
		treeMetas = append(treeMetas, collection.Meta{Name: full, Type: collection.TypeGeneric})
	}
	roots := viewmodel.BuildTree(treeMetas)
	existingSet := make(map[string]collection.Meta, len(existing))
	for name, meta := range existing {
		existingSet[name] = meta
	}
	var futureNode *viewmodel.ParsedCollection
	stack := append([]*viewmodel.ParsedCollection(nil), roots...)
	for len(stack) > 0 {
		last := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if last == nil {
			continue
		}
		if last.ID == "Future" {
			futureNode = last
			last.Type = collection.TypeMonthly
			last.Exists = true
		} else if last.ParentID == "Future" {
			last.Type = collection.TypeGeneric
			if _, ok := existingSet[last.ID]; ok {
				last.Exists = true
			} else {
				last.Exists = false
			}
		} else {
			last.Exists = false
		}
		if len(last.Children) > 0 {
			stack = append(stack, last.Children...)
		}
	}
	if futureNode != nil {
		monthMap := make(map[string]*viewmodel.ParsedCollection, len(futureNode.Children))
		for _, child := range futureNode.Children {
			if child == nil {
				continue
			}
			monthMap[child.Name] = child
		}
		ordered := make([]*viewmodel.ParsedCollection, 0, len(monthMap))
		baseMonth := startOfMonth(now)
		if baseMonth.IsZero() {
			baseMonth = startOfMonth(time.Now())
		}
		for i := 1; i <= 12; i++ {
			slot := baseMonth.AddDate(0, i, 0)
			name := slot.Format("January 2006")
			if child, ok := monthMap[name]; ok {
				child.Priority = i
				child.SortKey = fmt.Sprintf("%02d-%s", i, strings.ToLower(name))
				ordered = append(ordered, child)
			}
		}
		futureNode.Children = ordered
	}
	return roots
}

func filterMoveCollections(collections []*viewmodel.ParsedCollection) []*viewmodel.ParsedCollection {
	if len(collections) == 0 {
		return nil
	}
	trimmed := make([]*viewmodel.ParsedCollection, 0, len(collections))
	for _, col := range collections {
		if col == nil {
			continue
		}
		if isFutureCollection(col.ID) {
			continue
		}
		clone := cloneParsedCollection(col)
		clone.Children = filterMoveCollections(clone.Children)
		trimmed = append(trimmed, clone)
	}
	return trimmed
}

func cloneParsedCollection(col *viewmodel.ParsedCollection) *viewmodel.ParsedCollection {
	if col == nil {
		return nil
	}
	clone := *col
	if len(col.Children) > 0 {
		clone.Children = make([]*viewmodel.ParsedCollection, len(col.Children))
		for i := range col.Children {
			clone.Children[i] = cloneParsedCollection(col.Children[i])
		}
	}
	return &clone
}

func isFutureCollection(id string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(id))
	if trimmed == "" {
		return false
	}
	if trimmed == "future" {
		return true
	}
	return strings.HasPrefix(trimmed, "future/")
}

func (m *Model) loadBulletDetail(collectionID, bulletID, requestID string) tea.Cmd {
	svc := m.service
	return func() tea.Msg {
		if svc == nil {
			return bulletDetailLoadedMsg{requestID: requestID, err: fmt.Errorf("service unavailable")}
		}
		ctx := context.Background()
		entries, err := svc.Entries(ctx, collectionID)
		if err != nil {
			return bulletDetailLoadedMsg{requestID: requestID, err: err}
		}
		for _, e := range entries {
			if e == nil {
				continue
			}
			if strings.TrimSpace(e.ID) == bulletID {
				e.EnsureHistorySeed()
				return bulletDetailLoadedMsg{requestID: requestID, entry: e}
			}
		}
		return bulletDetailLoadedMsg{requestID: requestID, err: fmt.Errorf("entry not found")}
	}
}

func (m *Model) handleBulletDetailLoaded(msg bulletDetailLoadedMsg) tea.Cmd {
	if msg.requestID == "" || msg.requestID != m.detailLoadID {
		if msg.requestID != "" && msg.requestID == m.moveLoadID {
			return m.handleMoveDetailLoaded(msg)
		}
		return nil
	}
	if m.detailOverlay == nil {
		return nil
	}
	model := m.detailOverlay.Model()
	if model == nil {
		return nil
	}
	if msg.err != nil {
		model.SetError(msg.err)
		if m.command != nil {
			m.command.SetStatus("Bullet detail error: " + msg.err.Error())
		}
		return nil
	}
	if msg.entry == nil {
		model.SetError(fmt.Errorf("entry not available"))
		return nil
	}
	model.SetEntry(msg.entry)
	if m.command != nil {
		label := strings.TrimSpace(msg.entry.Message)
		if label == "" {
			label = msg.entry.ID
		}
		m.command.SetStatus("Loaded bullet details for " + label)
	}
	return nil
}

func (m *Model) handleMoveDetailLoaded(msg bulletDetailLoadedMsg) tea.Cmd {
	if m.moveOverlay == nil {
		return nil
	}
	model := m.moveOverlay.detail
	if model == nil {
		return nil
	}
	if msg.err != nil {
		model.SetError(msg.err)
		if m.command != nil {
			m.command.SetStatus("Bullet detail error: " + msg.err.Error())
		}
		return nil
	}
	if msg.entry == nil {
		model.SetError(fmt.Errorf("entry not available"))
		return nil
	}
	model.SetEntry(msg.entry)
	return nil
}

func (m *Model) handleMoveSelection(msg events.CollectionSelectMsg) tea.Cmd {
	if !m.moveVisible {
		return nil
	}
	target := resolvedCollectionPath(msg.Collection)
	if target == "" {
		return nil
	}
	bulletID := strings.TrimSpace(m.moveBulletID)
	if bulletID == "" {
		return m.closeMoveOverlayWithStatus("Move unavailable: no bullet selected")
	}
	if target == strings.TrimSpace(m.moveCollectionID) {
		return m.closeMoveOverlayWithStatus("Bullet already in selected collection")
	}
	if m.service == nil {
		if m.command != nil {
			m.command.SetStatus("Move failed: service offline")
		}
		return nil
	}
	ctx := context.Background()
	if m.moveFutureOnly && !msg.Exists {
		targetType := msg.Collection.Type
		if strings.HasPrefix(target, "Future/") {
			targetType = collection.TypeGeneric
		}
		if targetType == "" {
			switch {
			case strings.HasPrefix(target, "Future/"):
				targetType = collection.TypeGeneric
			case target == "Future":
				targetType = collection.TypeMonthly
			default:
				targetType = collection.TypeGeneric
			}
		}
		if err := m.service.EnsureCollectionOfType(ctx, target, targetType); err != nil {
			if m.command != nil {
				m.command.SetStatus("Move failed: " + err.Error())
			}
			return nil
		}
	}
	clone, err := m.service.Move(ctx, bulletID, target)
	if err != nil {
		if m.command != nil {
			m.command.SetStatus("Move failed: " + err.Error())
		}
		return nil
	}
	label := strings.TrimSpace(msg.Collection.Label())
	if label == "" {
		label = target
	}
	var cmds []tea.Cmd
	if m.journalNav != nil {
		ref := events.CollectionRef{ID: target}
		if idx := strings.LastIndex(target, "/"); idx >= 0 {
			ref.ParentID = target[:idx]
			ref.Name = strings.TrimSpace(target[idx+1:])
		} else {
			ref.Name = target
		}
		if cmd := m.journalNav.SelectCollection(ref); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if cache := m.journalCache; cache != nil {
		if cmd := m.collectionSyncCmd(target); cmd != nil {
			cmds = append(cmds, cmd)
		}
		origin := strings.TrimSpace(m.moveCollectionID)
		if origin == "" {
			origin = clone.Collection
		}
		if origin != "" {
			if cmd := m.collectionSyncCmd(origin); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	if cmd := m.closeMoveOverlayWithStatus("Moved bullet to " + label); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if !msg.Exists {
		if snap := m.snapshotSyncCmd(); snap != nil {
			cmds = append(cmds, snap)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func resolvedCollectionPath(ref events.CollectionRef) string {
	path := strings.TrimSpace(ref.ID)
	if path != "" {
		return path
	}
	name := strings.TrimSpace(ref.Name)
	parent := strings.TrimSpace(ref.ParentID)
	switch {
	case parent != "" && name != "":
		return strings.TrimSuffix(parent, "/") + "/" + name
	case name != "":
		return name
	default:
		return ""
	}
}

func (m *Model) handleBulletComplete(msg events.BulletCompleteMsg) tea.Cmd {
	id := strings.TrimSpace(msg.Bullet.ID)
	if id == "" {
		return nil
	}
	if m.service == nil {
		if m.command != nil {
			m.command.SetStatus("Complete unavailable: service offline")
		}
		return nil
	}
	ctx := context.Background()
	entry, err := m.service.Complete(ctx, id)
	if err != nil {
		if m.command != nil {
			m.command.SetStatus("Complete failed: " + err.Error())
		}
		return nil
	}
	if m.command != nil {
		label := strings.TrimSpace(msg.Bullet.Label)
		if label == "" && entry != nil {
			label = strings.TrimSpace(entry.Message)
		}
		if label == "" {
			label = id
		}
		m.command.SetStatus("Completed " + label)
	}
	return m.collectionSyncCmd(msg.Collection.ID)
}

func (m *Model) handleBulletStrike(msg events.BulletStrikeMsg) tea.Cmd {
	id := strings.TrimSpace(msg.Bullet.ID)
	if id == "" {
		return nil
	}
	if m.service == nil {
		if m.command != nil {
			m.command.SetStatus("Strike unavailable: service offline")
		}
		return nil
	}
	ctx := context.Background()
	entry, err := m.service.Strike(ctx, id)
	if err != nil {
		if m.command != nil {
			m.command.SetStatus("Strike failed: " + err.Error())
		}
		return nil
	}
	if m.command != nil {
		label := strings.TrimSpace(msg.Bullet.Label)
		if label == "" && entry != nil {
			label = strings.TrimSpace(entry.Message)
		}
		if label == "" {
			label = id
		}
		m.command.SetStatus("Marked irrelevant: " + label)
	}
	return m.collectionSyncCmd(msg.Collection.ID)
}

func (m *Model) handleBulletMoveFuture(msg events.BulletMoveFutureMsg) tea.Cmd {
	bulletID := strings.TrimSpace(msg.Bullet.ID)
	if bulletID == "" {
		return nil
	}
	if m.service == nil {
		if m.command != nil {
			m.command.SetStatus("Move unavailable: service offline")
		}
		return nil
	}
	collectionID := strings.TrimSpace(msg.Collection.ID)
	if collectionID == "" {
		collectionID = strings.TrimSpace(msg.Bullet.Note)
	}
	if collectionID == "" {
		if m.command != nil {
			m.command.SetStatus("Move unavailable: missing collection context")
		}
		return nil
	}
	ctx := context.Background()
	now := m.today
	if now.IsZero() {
		now = time.Now()
	}
	roots, err := m.futureMoveCollections(ctx, now)
	if err != nil {
		if m.command != nil {
			m.command.SetStatus("Move unavailable: " + err.Error())
		}
		return nil
	}
	if len(roots) == 0 {
		if m.command != nil {
			m.command.SetStatus("Move unavailable: Future view not ready")
		}
		return nil
	}
	title := strings.TrimSpace(msg.Collection.Title)
	if title == "" {
		title = collectionID
	}
	detailModel := bulletdetail.New(title, msg.Bullet.Label, collectionID, msg.Bullet.Note)
	nav := collectionnav.NewModel(roots)
	nav.SetBlurOnSelect(false)
	label := strings.TrimSpace(msg.Bullet.Label)
	if label == "" {
		label = bulletID
	}
	initial := events.CollectionRef{ID: "Future", Name: "Future"}
	if strings.HasPrefix(collectionID, "Future/") {
		initial.ID = collectionID
		name := strings.TrimPrefix(collectionID, "Future/")
		if name == "" {
			name = collectionID
		}
		initial.Name = name
	} else if collectionID == "Future" {
		initial.ID = collectionID
		initial.Name = "Future"
	}
	cfg := moveOverlayConfig{
		detail:       detailModel,
		nav:          nav,
		bulletID:     bulletID,
		collectionID: collectionID,
		label:        label,
		status:       "Choose Future destination for " + label,
		initialRef:   initial,
		futureOnly:   true,
		navOnRight:   false,
	}
	return m.openMoveOverlay(cfg)
}

func (m *Model) handleBulletSignifier(msg events.BulletSignifierMsg) tea.Cmd {
	id := strings.TrimSpace(msg.Bullet.ID)
	if id == "" {
		return nil
	}
	if m.service == nil {
		if m.command != nil {
			m.command.SetStatus("Signifier change unavailable: service offline")
		}
		return nil
	}
	ctx := context.Background()
	var (
		entry *entry.Entry
		err   error
	)
	if msg.Signifier == glyph.None {
		entry, err = m.service.ToggleSignifier(ctx, id, glyph.None)
	} else {
		entry, err = m.service.SetSignifier(ctx, id, msg.Signifier)
	}
	if err != nil {
		if m.command != nil {
			m.command.SetStatus("Signifier change failed: " + err.Error())
		}
		return nil
	}
	if m.command != nil {
		label := strings.TrimSpace(msg.Bullet.Label)
		if label == "" && entry != nil {
			label = strings.TrimSpace(entry.Message)
		}
		if label == "" {
			label = id
		}
		desc := "cleared signifier"
		if msg.Signifier != glyph.None {
			if info, ok := glyph.DefaultSignifiers()[msg.Signifier]; ok {
				desc = "set signifier " + strings.TrimSpace(info.Symbol+" "+info.Meaning)
			} else {
				desc = "set signifier"
			}
		}
		m.command.SetStatus(desc + " for " + label)
	}
	return m.collectionSyncCmd(msg.Collection.ID)
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
	if m.addVisible {
		if cmd := m.closeAddTaskOverlay(); cmd != nil {
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
		snapshot, err := cachepkg.BuildSnapshot(context.Background(), svc)
		return journalLoadedMsg{snapshot: snapshot, err: err}
	}
}

func (m *Model) scheduleDayCheck() tea.Cmd {
	return tea.Tick(dayCheckInterval, func(time.Time) tea.Msg {
		return dayCheckMsg{}
	})
}

func (m *Model) refreshToday(now time.Time) {
	day := startOfDay(now)
	if !m.today.IsZero() && m.today.Equal(day) {
		return
	}
	m.today = day
	if m.journalNav != nil {
		m.journalNav.SetNow(now)
	}
	m.layoutContent()
}

func cacheListenCmd(cache *cachepkg.Cache) tea.Cmd {
	if cache == nil {
		return nil
	}
	ch := cache.Events()
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return cacheMsg{payload: msg}
	}
}

func startWatchCmd(parent context.Context, svc *app.Service) tea.Cmd {
	if svc == nil || parent == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(parent)
		ch, err := svc.Watch(ctx)
		if err != nil {
			cancel()
			return watchStartedMsg{err: err}
		}
		return watchStartedMsg{ch: ch, cancel: cancel}
	}
}

func (m *Model) waitForWatch() tea.Cmd {
	if m.watchCh == nil {
		return nil
	}
	ch := m.watchCh
	return func() tea.Msg {
		if ev, ok := <-ch; ok {
			return watchEventMsg{event: ev}
		}
		return watchStoppedMsg{}
	}
}

func (m *Model) stopWatch() {
	if m.watchCancel != nil {
		m.watchCancel()
		m.watchCancel = nil
	}
	m.watchCh = nil
}

func (m *Model) handleWatchEvent(ev store.Event) tea.Cmd {
	switch ev.Type {
	case store.EventCollectionChanged:
		if strings.HasPrefix(ev.Collection, "fromCollection:") {
			return nil
		}
		return m.collectionSyncCmd(ev.Collection)
	case store.EventCollectionsInvalidated:
		return m.snapshotSyncCmd()
	default:
		if strings.HasPrefix(ev.Collection, "fromCollection:") {
			return nil
		}
		return m.snapshotSyncCmd()
	}
}

func (m *Model) collectionSyncCmd(collectionID string) tea.Cmd {
	cache := m.journalCache
	if cache == nil {
		return nil
	}
	return func() tea.Msg {
		if err := cache.SyncCollection(context.Background(), collectionID); err != nil {
			return watchErrorMsg{err: err}
		}
		return nil
	}
}

func (m *Model) snapshotSyncCmd() tea.Cmd {
	cache := m.journalCache
	svc := m.service
	if cache == nil || svc == nil {
		return nil
	}
	return func() tea.Msg {
		snapshot, err := cachepkg.BuildSnapshot(context.Background(), svc)
		if err != nil {
			return watchErrorMsg{err: err}
		}
		cache.ApplySnapshot(snapshot)
		return nil
	}
}

func startOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func startOfMonth(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	year, month, _ := t.Date()
	return time.Date(year, month, 1, 0, 0, 0, 0, t.Location())
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
	var cmds []tea.Cmd
	if m.addVisible {
		if cmd := m.closeAddTaskOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.detailVisible {
		if cmd := m.closeBulletDetailOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
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

func (m *Model) openAddTaskOverlay(opts addtask.Options, req events.AddTaskRequestMsg) tea.Cmd {
	if m.overlayPane == nil {
		m.overlayPane = overlaypane.New(m.width, maxInt(1, m.height-1))
	}
	var cmds []tea.Cmd
	if m.helpVisible {
		if cmd := m.closeHelpOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.reportVisible {
		if cmd := m.closeReportOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.addVisible {
		if cmd := m.closeAddTaskOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	model := addtask.NewModel(m.journalCache, opts)
	model.SetID(addTaskOverlayID)
	wrapper := newAddtaskOverlay(model)
	placement := command.OverlayPlacement{Fullscreen: true}
	if cmd := m.overlayPane.SetOverlay(wrapper, placement); cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.addOverlay = wrapper
	m.addVisible = true
	_ = m.dropFocusKind(focusKindCommand)
	m.pushFocus(focusTarget{kind: focusKindOverlay, overlay: overlayKindAdd})
	cmds = append(cmds, m.blurJournalPanes()...)
	if focusCmd := m.overlayPane.Focus(); focusCmd != nil {
		cmds = append(cmds, focusCmd)
	}
	label := strings.TrimSpace(req.CollectionLabel)
	if label == "" {
		label = req.CollectionID
	}
	if m.command != nil {
		m.command.SetStatus("Add task overlay opened for " + label)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) closeAddTaskOverlay() tea.Cmd {
	if !m.addVisible {
		return nil
	}
	var cmds []tea.Cmd
	if m.overlayPane != nil {
		if cmd := m.overlayPane.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.overlayPane.ClearOverlay()
	}
	m.addOverlay = nil
	m.addVisible = false
	_, _ = m.popFocusKind(focusKindOverlay)
	if cmd := m.restoreFocusAfterOverlay(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if m.command != nil {
		m.command.SetStatus("Add task overlay closed")
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) closeBulletDetailOverlay() tea.Cmd {
	if !m.detailVisible {
		return nil
	}
	var cmds []tea.Cmd
	if m.overlayPane != nil {
		if cmd := m.overlayPane.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.overlayPane.ClearOverlay()
	}
	m.detailOverlay = nil
	m.detailVisible = false
	m.detailLoadID = ""
	_, _ = m.popFocusKind(focusKindOverlay)
	if cmd := m.restoreFocusAfterOverlay(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if m.command != nil {
		m.command.SetStatus("Bullet detail overlay closed")
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) closeMoveOverlay() tea.Cmd {
	return m.closeMoveOverlayWithStatus("")
}

func (m *Model) closeMoveOverlayWithStatus(status string) tea.Cmd {
	if !m.moveVisible {
		if status != "" && m.command != nil {
			m.command.SetStatus(status)
		}
		return nil
	}
	var cmds []tea.Cmd
	if m.overlayPane != nil {
		if cmd := m.overlayPane.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.overlayPane.ClearOverlay()
	}
	m.moveOverlay = nil
	m.moveVisible = false
	m.moveLoadID = ""
	m.moveBulletID = ""
	m.moveCollectionID = ""
	m.moveFutureOnly = false
	_, _ = m.popFocusKind(focusKindOverlay)
	if cmd := m.restoreFocusAfterOverlay(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if status != "" && m.command != nil {
		m.command.SetStatus(status)
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
	if m.addVisible {
		return m.closeAddTaskOverlay()
	}
	if m.detailVisible {
		return m.closeBulletDetailOverlay()
	}
	if m.moveVisible {
		return m.closeMoveOverlay()
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

func (m *Model) handleBlurMsg(msg events.BlurMsg) tea.Cmd {
	if msg.Component == addTaskOverlayID && m.addVisible {
		return m.closeAddTaskOverlay()
	}
	if msg.Component == bulletDetailOverlayID && m.detailVisible {
		return m.closeBulletDetailOverlay()
	}
	if msg.Component == moveNavID && m.moveVisible {
		return m.closeMoveOverlay()
	}
	return nil
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
