package app

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

const statusClearTimeout = 5 * time.Second

type reportLoadedMsg struct {
	result app.ReportResult
	err    error
}

type statusClearMsg struct {
	token int64
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
	migrateVisible   bool
	migrateOverlay   *migrationOverlay
	migrateWindow    migrationWindow

	statusText         string
	statusClearPending bool
	statusClearActive  bool
	statusClearToken   int64

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
	overlayKindMigrate
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
		{Name: "today", Description: "Jump to today's collection"},
		{Name: "future", Description: "Jump to the Future log"},
		{Name: "help", Description: "Show command tips"},
		{Name: "lock", Description: "Lock the selected task"},
		{Name: "unlock", Description: "Unlock the selected task"},
		{Name: "quit", Description: "Exit bujo"},
		{Name: "report", Description: "Show completed entries report"},
		{Name: "debug", Description: "Toggle debug event viewer"},
		{Name: "migrate", Description: "Review and migrate open tasks"},
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

func (m *Model) closeMigrateOverlay() tea.Cmd {
	return m.closeMigrateOverlayWithStatus("")
}

func (m *Model) closeMigrateOverlayWithStatus(status string) tea.Cmd {
	if !m.migrateVisible {
		if status != "" && m.command != nil {
			m.setStatus(status)
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
	m.migrateOverlay = nil
	m.migrateVisible = false
	m.migrateWindow = migrationWindow{}
	_, _ = m.popFocusKind(focusKindOverlay)
	if cmd := m.restoreFocusAfterOverlay(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if status != "" && m.command != nil {
		m.setStatus(status)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// Update routes Bubble Tea messages to composed components and returns the updated model with any commands to execute.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.noteEvent(msg)

	var cmds []tea.Cmd
	skipCommandUpdate := false
	skipJournalKey := false

	if m.dump != nil {
		_, _ = fmt.Fprintf(m.dump, "%s ", time.Now().Format("2006-01-02T15:04:05"))
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
				m.setStatus("Watch start failed: " + v.err.Error())
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
	case statusClearMsg:
		if v.token == m.statusClearToken && m.statusClearActive {
			m.clearStatus()
		}
	case watchErrorMsg:
		if v.err != nil {
			if m.command != nil {
				m.setStatus("Watch error: " + v.err.Error())
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
		if m.commandActive {
			skipJournalKey = true
		}
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
				m.setStatus("Commands: :quit, :today, :future, :debug, :report [window], :migrate [window], :lock, :unlock, :help")
				break
			}
			parts := strings.Fields(raw)
			if len(parts) == 0 {
				m.setStatus("Commands: :quit, :today, :future, :debug, :report [window], :migrate [window], :lock, :unlock, :help")
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
					m.setStatus("Help overlay opened (Esc or : to close)")
				case "closed":
					m.setStatus("Help overlay closed")
				case "noop":
					m.setStatus("Help unavailable")
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
					m.setStatus("Report overlay opened")
				case "closed":
					m.setStatus("Report overlay closed")
				case "error":
					// status set inside showReportOverlay
				}
				m.layoutContent()
			case "migrate":
				cmd, state := m.showMigrateOverlay(arg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
				switch state {
				case "opened":
					m.setStatus("Migration overlay opened")
				case "closed":
					m.setStatus("Migration overlay closed")
				case "error":
					// status set within showMigrateOverlay
				case "noop":
					// no change
				}
				m.layoutContent()
			case "today":
				if cmd := m.jumpToToday(true); cmd != nil {
					cmds = append(cmds, cmd)
				}
			case "future":
				if cmd := m.jumpToFuture(true); cmd != nil {
					cmds = append(cmds, cmd)
				}
			case "lock":
				if cmd := m.lockSelectedBullet(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			case "unlock":
				if cmd := m.unlockSelectedBullet(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			default:
				m.setStatus("Unhandled command: " + cmdName)
			}
			_ = m.dropFocusKind(focusKindCommand)
		}
	case events.CommandCancelMsg:
		if m.command != nil && v.Component == m.command.ID() {
			m.setStatus("Ready")
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
					if m.overlayPane != nil && m.overlayPane.HasOverlay() {
						if cmd := m.overlayPane.Blur(); cmd != nil {
							cmds = append(cmds, cmd)
						}
					}
					m.pushFocus(focusTarget{kind: focusKindCommand})
				}
			} else {
				if m.commandActive {
					m.commandActive = false
					if m.overlayPane != nil && m.overlayPane.HasOverlay() {
						if cmd := m.overlayPane.Focus(); cmd != nil {
							cmds = append(cmds, cmd)
						}
					} else if !m.helpVisible {
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
		if m.migrateVisible {
			if cmd := m.handleMigrationMoveRequest(v); cmd != nil {
				cmds = append(cmds, cmd)
			}
			skipCommandUpdate = true
			skipJournalKey = true
			break
		}
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
		if m.migrateVisible {
			if cmd := m.handleMigrationMoveFuture(v); cmd != nil {
				cmds = append(cmds, cmd)
			}
			skipCommandUpdate = true
			skipJournalKey = true
			break
		}
		if cmd := m.handleBulletMoveFuture(v); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case events.BulletSignifierMsg:
		if cmd := m.handleBulletSignifier(v); cmd != nil {
			cmds = append(cmds, cmd)
		}
		skipJournalKey = true
		skipCommandUpdate = true
	case events.BulletSelectMsg:
		if m.migrateVisible && v.Component == migrateDetailID {
			if cmd := m.handleMigrationKeep(v); cmd != nil {
				cmds = append(cmds, cmd)
			}
			skipJournalKey = true
			skipCommandUpdate = true
		}
	case migrationCreateCollectionMsg:
		if cmd := m.handleMigrationCreateCollection(v.Name); cmd != nil {
			cmds = append(cmds, cmd)
		}
		skipJournalKey = true
		skipCommandUpdate = true
	case migrationCreateCollectionCancelledMsg:
		if m.command != nil {
			m.setStatus("New collection creation cancelled")
		}
		if m.migrateOverlay != nil {
			if cmd := m.migrateOverlay.FocusDetail(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		skipJournalKey = true
		skipCommandUpdate = true
	case bulletDetailLoadedMsg:
		if cmd := m.handleBulletDetailLoaded(v); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case events.CollectionSelectMsg:
		if m.migrateVisible && m.migrateOverlay != nil {
			if cmd := m.handleMigrationCollectionSelect(v); cmd != nil {
				cmds = append(cmds, cmd)
			}
			skipCommandUpdate = true
			skipJournalKey = true
			break
		}
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
			m.setStatus("Report overlay closed")
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
				m.setStatus("Journal load failed: " + v.err.Error())
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
		if cmd := m.jumpToToday(false); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if m.command != nil {
			m.setStatus("Journal loaded")
		}
		m.layoutContent()
	case events.FocusMsg:
		m.handleFocusMsg(v)
	case events.BlurMsg:
		if cmd := m.handleBlurMsg(v); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if post := m.postInteractionStatus(msg); post != nil {
		cmds = append(cmds, post)
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
	_, _ = fmt.Fprintf(m.dump, "%s %s\n", time.Now().Format("2006-01-02T15:04:05"), fmt.Sprintf(format, args...))
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
			m.setStatus("Debug log hidden")
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
		m.setStatus("Debug log visible")
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
			m.setStatus("Add task unavailable: missing collection")
		}
		return nil
	}
	if m.journalCache == nil {
		if m.command != nil {
			m.setStatus("Add task unavailable: journal cache offline")
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
			m.setStatus("Bullet details unavailable: missing bullet ID")
		}
		return nil
	}
	if m.service == nil {
		if m.command != nil {
			m.setStatus("Bullet details unavailable: service offline")
		}
		return nil
	}
	collectionID := strings.TrimSpace(msg.Collection.ID)
	if collectionID == "" {
		collectionID = strings.TrimSpace(msg.Bullet.Note)
	}
	if collectionID == "" {
		if m.command != nil {
			m.setStatus("Bullet details unavailable: missing collection context")
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
		m.setStatus("Loading details for " + statusLabel)
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
			m.setStatus("Move unavailable: missing bullet ID")
		}
		return nil
	}
	if m.service == nil {
		if m.command != nil {
			m.setStatus("Move unavailable: service offline")
		}
		return nil
	}
	if m.journalCache == nil {
		if m.command != nil {
			m.setStatus("Move unavailable: journal cache offline")
		}
		return nil
	}
	collectionID := strings.TrimSpace(msg.Collection.ID)
	if collectionID == "" {
		collectionID = strings.TrimSpace(msg.Bullet.Note)
	}
	if collectionID == "" {
		if m.command != nil {
			m.setStatus("Move unavailable: missing collection context")
		}
		return nil
	}
	snapshot := m.journalCache.Snapshot()
	if len(snapshot.Collections) == 0 || len(snapshot.Sections) == 0 {
		if m.command != nil {
			m.setStatus("Move unavailable: no collections")
		}
		return nil
	}
	if source := findBulletInSnapshot(snapshot.Sections, collectionID, bulletID); source != nil && source.Locked {
		if m.command != nil {
			m.setStatus("Move unavailable: bullet is locked")
		}
		return nil
	}
	trimmedCollections := filterMoveCollections(snapshot.Collections)
	if len(trimmedCollections) == 0 {
		if m.command != nil {
			m.setStatus("Move unavailable: no target collections")
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
	mOverlay := newMovebulletOverlay(cfg.detail, cfg.nav, cfg.navOnRight, m.dump)
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
		m.setStatus(status)
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
			m.setStatus("Bullet detail error: " + msg.err.Error())
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
		m.setStatus("Loaded bullet details for " + label)
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
			m.setStatus("Bullet detail error: " + msg.err.Error())
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
			m.setStatus("Move failed: service offline")
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
				m.setStatus("Move failed: " + err.Error())
			}
			return nil
		}
	}
	clone, err := m.service.Move(ctx, bulletID, target)
	if err != nil {
		if m.command != nil {
			m.setStatus("Move failed: " + err.Error())
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
			m.setStatus("Complete unavailable: service offline")
		}
		return nil
	}
	ctx := context.Background()
	entry, err := m.service.Complete(ctx, id)
	if err != nil {
		if m.command != nil {
			m.setStatus("Complete failed: " + err.Error())
		}
		return nil
	}
	label := strings.TrimSpace(msg.Bullet.Label)
	if label == "" && entry != nil {
		label = strings.TrimSpace(entry.Message)
	}
	if label == "" {
		label = id
	}
	status := "Completed " + label
	if m.command != nil {
		m.setStatus(status)
	}
	var cmds []tea.Cmd
	if cmd := m.removeMigrationBullet(id, status); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if sync := m.collectionSyncCmd(msg.Collection.ID); sync != nil {
		cmds = append(cmds, sync)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) handleBulletStrike(msg events.BulletStrikeMsg) tea.Cmd {
	id := strings.TrimSpace(msg.Bullet.ID)
	if id == "" {
		return nil
	}
	if m.service == nil {
		if m.command != nil {
			m.setStatus("Strike unavailable: service offline")
		}
		return nil
	}
	ctx := context.Background()
	entry, err := m.service.Strike(ctx, id)
	if err != nil {
		if m.command != nil {
			m.setStatus("Strike failed: " + err.Error())
		}
		return nil
	}
	label := strings.TrimSpace(msg.Bullet.Label)
	if label == "" && entry != nil {
		label = strings.TrimSpace(entry.Message)
	}
	if label == "" {
		label = id
	}
	status := "Marked irrelevant: " + label
	if m.command != nil {
		m.setStatus(status)
	}
	var cmds []tea.Cmd
	if cmd := m.removeMigrationBullet(id, status); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if sync := m.collectionSyncCmd(msg.Collection.ID); sync != nil {
		cmds = append(cmds, sync)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) handleBulletMoveFuture(msg events.BulletMoveFutureMsg) tea.Cmd {
	bulletID := strings.TrimSpace(msg.Bullet.ID)
	if bulletID == "" {
		return nil
	}
	if m.service == nil {
		if m.command != nil {
			m.setStatus("Move unavailable: service offline")
		}
		return nil
	}
	collectionID := strings.TrimSpace(msg.Collection.ID)
	if collectionID == "" {
		collectionID = strings.TrimSpace(msg.Bullet.Note)
	}
	if collectionID == "" {
		if m.command != nil {
			m.setStatus("Move unavailable: missing collection context")
		}
		return nil
	}
	snapshot := m.journalCache.Snapshot()
	if len(snapshot.Collections) == 0 || len(snapshot.Sections) == 0 {
		if m.command != nil {
			m.setStatus("Move unavailable: no collections")
		}
		return nil
	}
	if source := findBulletInSnapshot(snapshot.Sections, collectionID, bulletID); source != nil && source.Locked {
		if m.command != nil {
			m.setStatus("Move unavailable: bullet is locked")
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
			m.setStatus("Move unavailable: " + err.Error())
		}
		return nil
	}
	if len(roots) == 0 {
		if m.command != nil {
			m.setStatus("Move unavailable: Future view not ready")
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
			m.setStatus("Signifier change unavailable: service offline")
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
			m.setStatus("Signifier change failed: " + err.Error())
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
		m.setStatus(desc + " for " + label)
	}
	return m.collectionSyncCmd(msg.Collection.ID)
}

func (m *Model) lockSelectedBullet() tea.Cmd {
	m.setStatus("")
	if m.service == nil {
		m.setStatus("Lock unavailable: service offline")
		return nil
	}
	if m.journalView == nil {
		m.setStatus("Lock unavailable: journal cache offline")
		return nil
	}
	section, bullet, ok := m.journalView.CurrentSelection()
	if !ok {
		m.setStatus("Lock unavailable: select a task")
		return nil
	}
	collectionID := strings.TrimSpace(section.ID)
	bulletID := strings.TrimSpace(bullet.ID)
	if collectionID == "" || bulletID == "" {
		m.setStatus("Lock unavailable: select a task")
		return nil
	}
	if bullet.Locked {
		label := strings.TrimSpace(bullet.Label)
		if label == "" {
			label = bulletID
		}
		m.setStatus(label + " is already locked")
		return nil
	}
	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := m.service.Lock(ctx, bulletID); err != nil {
		m.setStatus("Lock failed: " + err.Error())
		return nil
	}
	label := strings.TrimSpace(bullet.Label)
	if label == "" {
		label = bulletID
	}
	m.setStatus("Locked " + label)
	return m.collectionSyncCmd(collectionID)
}

func (m *Model) unlockSelectedBullet() tea.Cmd {
	m.setStatus("")
	if m.service == nil {
		m.setStatus("Unlock unavailable: service offline")
		return nil
	}
	if m.journalView == nil {
		m.setStatus("Unlock unavailable: journal cache offline")
		return nil
	}
	section, bullet, ok := m.journalView.CurrentSelection()
	if !ok {
		m.setStatus("Unlock unavailable: select a task")
		return nil
	}
	collectionID := strings.TrimSpace(section.ID)
	bulletID := strings.TrimSpace(bullet.ID)
	if collectionID == "" || bulletID == "" {
		m.setStatus("Unlock unavailable: select a task")
		return nil
	}
	if !bullet.Locked {
		label := strings.TrimSpace(bullet.Label)
		if label == "" {
			label = bulletID
		}
		m.setStatus(label + " is not locked")
		return nil
	}
	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := m.service.Unlock(ctx, bulletID); err != nil {
		m.setStatus("Unlock failed: " + err.Error())
		return nil
	}
	label := strings.TrimSpace(bullet.Label)
	if label == "" {
		label = bulletID
	}
	m.setStatus("Unlocked " + label)
	return m.collectionSyncCmd(collectionID)
}

func (m *Model) jumpToToday(showStatus bool) tea.Cmd {
	if m.journalNav == nil {
		if showStatus {
			m.setStatus("Today unavailable: journal not ready")
		}
		return nil
	}
	now := time.Now()
	if !m.today.IsZero() {
		now = m.today
	}
	ref, _ := todayCollectionRefFromCache(m.journalCache, now)
	if ref.ID == "" {
		if showStatus {
			m.setStatus("Today collection unavailable")
		}
		return nil
	}
	var cmds []tea.Cmd
	if cmd := m.collectionSyncCmd(ref.ID); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.journalNav.SelectCollection(ref); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if m.journalView != nil {
		if cmd := m.journalView.FocusDetail(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.journalDetail != nil {
		m.journalDetail.FocusCollection(ref.ID)
	}
	if showStatus {
		label := strings.TrimSpace(ref.Name)
		if label == "" {
			label = "Today"
		}
		m.setStatus("Selected Today (" + label + ")")
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) handleMigrationMoveRequest(msg events.MoveBulletRequestMsg) tea.Cmd {
	if !m.migrateVisible || m.migrateOverlay == nil {
		return nil
	}
	bulletID := strings.TrimSpace(msg.Bullet.ID)
	if bulletID == "" {
		return nil
	}
	targetRef, exists, ok := m.migrateOverlay.TargetSelection()
	if !ok {
		if m.command != nil {
			m.setStatus("Move unavailable: select a destination")
		}
		return nil
	}
	targetPath := strings.TrimSpace(resolvedCollectionPath(targetRef))
	if targetPath == "" {
		targetPath = strings.TrimSpace(targetRef.Name)
	}
	if targetPath == "" {
		if m.command != nil {
			m.setStatus("Move unavailable: select a destination")
		}
		return nil
	}
	origin := strings.TrimSpace(msg.Collection.ID)
	if origin == "" {
		origin = strings.TrimSpace(msg.Bullet.Note)
	}
	label := strings.TrimSpace(msg.Bullet.Label)
	if label == "" {
		label = bulletID
	}
	if targetPath == origin {
		status := "Kept " + label
		return m.removeMigrationBullet(bulletID, status)
	}
	if m.service == nil {
		if m.command != nil {
			m.setStatus("Move unavailable: service offline")
		}
		return nil
	}
	ctx := context.Background()
	if !exists {
		targetType := targetRef.Type
		if targetType == "" {
			targetType = collection.TypeGeneric
		}
		if err := m.service.EnsureCollectionOfType(ctx, targetPath, targetType); err != nil {
			if m.command != nil {
				m.setStatus("Move failed: " + err.Error())
			}
			return nil
		}
	}
	clone, err := m.service.Move(ctx, bulletID, targetPath)
	if err != nil {
		if m.command != nil {
			m.setStatus("Move failed: " + err.Error())
		}
		return nil
	}
	if strings.TrimSpace(label) == "" && clone != nil {
		label = strings.TrimSpace(clone.Message)
		if label == "" {
			label = bulletID
		}
	}
	destination := targetRef.Label()
	if strings.TrimSpace(destination) == "" {
		destination = targetPath
	}
	status := "Moved " + label + " to " + destination
	var cmds []tea.Cmd
	if cmd := m.removeMigrationBullet(bulletID, status); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if sync := m.collectionSyncCmd(targetPath); sync != nil {
		cmds = append(cmds, sync)
	}
	if origin != "" && origin != targetPath {
		if sync := m.collectionSyncCmd(origin); sync != nil {
			cmds = append(cmds, sync)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) handleMigrationMoveFuture(msg events.BulletMoveFutureMsg) tea.Cmd {
	if !m.migrateVisible || m.migrateOverlay == nil {
		return nil
	}
	bulletID := strings.TrimSpace(msg.Bullet.ID)
	if bulletID == "" {
		return nil
	}
	targetRef, exists, ok := m.migrateOverlay.FutureSelection()
	if !ok {
		targetRef = events.CollectionRef{ID: "Future", Name: "Future", Type: collection.TypeMonthly}
		exists = true
	}
	targetPath := strings.TrimSpace(resolvedCollectionPath(targetRef))
	if targetPath == "" {
		targetPath = "Future"
	}
	origin := strings.TrimSpace(msg.Collection.ID)
	if origin == "" {
		origin = strings.TrimSpace(msg.Bullet.Note)
	}
	label := strings.TrimSpace(msg.Bullet.Label)
	if label == "" {
		label = bulletID
	}
	if targetPath == origin {
		status := "Kept " + label
		return m.removeMigrationBullet(bulletID, status)
	}
	if m.service == nil {
		if m.command != nil {
			m.setStatus("Move unavailable: service offline")
		}
		return nil
	}
	ctx := context.Background()
	if !exists {
		targetType := targetRef.Type
		if strings.HasPrefix(targetPath, "Future/") {
			if targetType == "" {
				targetType = collection.TypeGeneric
			}
		} else if targetType == "" {
			if targetPath == "Future" {
				targetType = collection.TypeMonthly
			} else {
				targetType = collection.TypeGeneric
			}
		}
		if err := m.service.EnsureCollectionOfType(ctx, targetPath, targetType); err != nil {
			if m.command != nil {
				m.setStatus("Move failed: " + err.Error())
			}
			return nil
		}
	}
	clone, err := m.service.Move(ctx, bulletID, targetPath)
	if err != nil {
		if m.command != nil {
			m.setStatus("Move failed: " + err.Error())
		}
		return nil
	}
	if strings.TrimSpace(label) == "" && clone != nil {
		label = strings.TrimSpace(clone.Message)
		if label == "" {
			label = bulletID
		}
	}
	dest := targetRef.Label()
	if strings.TrimSpace(dest) == "" {
		dest = targetPath
	}
	status := "Moved " + label + " to " + dest
	var cmds []tea.Cmd
	if cmd := m.removeMigrationBullet(bulletID, status); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if sync := m.collectionSyncCmd(targetPath); sync != nil {
		cmds = append(cmds, sync)
	}
	if origin != "" && origin != targetPath {
		if sync := m.collectionSyncCmd(origin); sync != nil {
			cmds = append(cmds, sync)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) handleMigrationKeep(msg events.BulletSelectMsg) tea.Cmd {
	if !m.migrateVisible || m.migrateOverlay == nil {
		return nil
	}
	if !msg.Exists {
		return nil
	}
	bulletID := strings.TrimSpace(msg.Bullet.ID)
	if bulletID == "" {
		return nil
	}
	label := strings.TrimSpace(msg.Bullet.Label)
	if label == "" {
		label = bulletID
	}
	status := "Kept " + label
	return m.removeMigrationBullet(bulletID, status)
}

func (m *Model) removeMigrationBullet(id, status string) tea.Cmd {
	id = strings.TrimSpace(id)
	if id == "" {
		if status != "" && m.command != nil {
			m.setStatus(status)
		}
		return nil
	}
	if !m.migrateVisible || m.migrateOverlay == nil {
		if status != "" && m.command != nil {
			m.setStatus(status)
		}
		return nil
	}
	m.migrateOverlay.RemoveBullet(id)
	if m.migrateOverlay.IsEmpty() {
		final := status
		if strings.TrimSpace(final) == "" {
			final = "Migration complete"
		}
		return m.closeMigrateOverlayWithStatus(final)
	}
	if status != "" && m.command != nil {
		m.setStatus(status)
	}
	if cmd := m.migrateOverlay.FocusDetail(); cmd != nil {
		return cmd
	}
	return nil
}

func (m *Model) handleMigrationCollectionSelect(msg events.CollectionSelectMsg) tea.Cmd {
	if m.migrateOverlay == nil {
		return nil
	}
	if msg.Component == migrateTargetNavID && strings.EqualFold(strings.TrimSpace(msg.Collection.ID), migrationNewCollectionID) {
		return m.startMigrationNewCollectionPrompt()
	}
	section, bulletRow, item, ok := m.migrateOverlay.CurrentMigrationSelection()
	if !ok || item == nil || item.Candidate.Entry == nil {
		return m.migrateOverlay.FocusDetail()
	}
	label := strings.TrimSpace(bulletRow.Label)
	if label == "" {
		label = strings.TrimSpace(item.Candidate.Entry.Message)
	}
	if label == "" {
		label = bulletRow.ID
	}
	sectionRef := events.CollectionViewRef{
		ID:       section.ID,
		Title:    section.Title,
		Subtitle: section.Subtitle,
	}
	bulletRef := events.BulletRef{
		ID:        bulletRow.ID,
		Label:     label,
		Note:      item.SectionID,
		Bullet:    bulletRow.Bullet,
		Signifier: bulletRow.Signifier,
	}
	switch msg.Component {
	case migrateFutureNavID:
		return m.handleMigrationMoveFuture(events.BulletMoveFutureMsg{
			Component:  migrateDetailID,
			Collection: sectionRef,
			Bullet:     bulletRef,
		})
	case migrateTargetNavID:
		return m.handleMigrationMoveRequest(events.MoveBulletRequestMsg{
			Component:  migrateDetailID,
			Collection: sectionRef,
			Bullet:     bulletRef,
		})
	default:
		return m.migrateOverlay.FocusDetail()
	}
}

func (m *Model) startMigrationNewCollectionPrompt() tea.Cmd {
	if m.migrateOverlay == nil {
		return nil
	}
	if m.command != nil {
		m.setStatus("Enter a name for the new collection")
	}
	return m.migrateOverlay.BeginNewCollectionPrompt()
}

func (m *Model) handleMigrationCreateCollection(name string) tea.Cmd {
	name = strings.TrimSpace(name)
	if name == "" {
		if m.command != nil {
			m.setStatus("Collection name cannot be empty")
		}
		if m.migrateOverlay != nil {
			return m.migrateOverlay.FocusDetail()
		}
		return nil
	}
	if m.service == nil {
		if m.command != nil {
			m.setStatus("Create failed: service offline")
		}
		if m.migrateOverlay != nil {
			return m.migrateOverlay.FocusDetail()
		}
		return nil
	}
	ctx := context.Background()
	if err := m.service.EnsureCollectionOfType(ctx, name, collection.TypeGeneric); err != nil {
		if m.command != nil {
			m.setStatus("Create failed: " + err.Error())
		}
		if m.migrateOverlay != nil {
			return m.migrateOverlay.FocusDetail()
		}
		return nil
	}
	var cmds []tea.Cmd
	if cmd := m.collectionSyncCmd(name); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if m.command != nil {
		m.setStatus("Created " + name)
	}
	if m.migrateOverlay == nil {
		if len(cmds) == 0 {
			return nil
		}
		return tea.Batch(cmds...)
	}
	_, bulletRow, item, ok := m.migrateOverlay.CurrentMigrationSelection()
	if !ok || item == nil || item.Candidate.Entry == nil {
		if focus := m.migrateOverlay.FocusDetail(); focus != nil {
			cmds = append(cmds, focus)
		}
		if len(cmds) == 0 {
			return nil
		}
		return tea.Batch(cmds...)
	}
	bulletID := strings.TrimSpace(item.Candidate.Entry.ID)
	if bulletID == "" {
		if focus := m.migrateOverlay.FocusDetail(); focus != nil {
			cmds = append(cmds, focus)
		}
		if len(cmds) == 0 {
			return nil
		}
		return tea.Batch(cmds...)
	}
	clone, err := m.service.Move(ctx, bulletID, name)
	if err != nil {
		if m.command != nil {
			m.setStatus("Move failed: " + err.Error())
		}
		if focus := m.migrateOverlay.FocusDetail(); focus != nil {
			cmds = append(cmds, focus)
		}
		if len(cmds) == 0 {
			return nil
		}
		return tea.Batch(cmds...)
	}
	label := strings.TrimSpace(bulletRow.Label)
	if label == "" && clone != nil {
		label = strings.TrimSpace(clone.Message)
	}
	if label == "" {
		label = bulletID
	}
	status := "Moved " + label + " to " + name
	if cmd := m.removeMigrationBullet(bulletID, status); cmd != nil {
		cmds = append(cmds, cmd)
	}
	origin := strings.TrimSpace(item.Candidate.Entry.Collection)
	if origin != "" && !strings.EqualFold(origin, name) {
		if cmd := m.collectionSyncCmd(origin); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) jumpToFuture(showStatus bool) tea.Cmd {
	if m.journalNav == nil {
		if showStatus {
			m.setStatus("Future collection unavailable")
		}
		return nil
	}
	ref := events.CollectionRef{ID: "Future", Name: "Future", Type: collection.TypeMonthly}
	var cmds []tea.Cmd
	if cmd := m.collectionSyncCmd(ref.ID); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.journalNav.SelectCollection(ref); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if m.journalView != nil {
		if cmd := m.journalView.FocusDetail(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.journalDetail != nil {
		m.journalDetail.FocusCollection(ref.ID)
	}
	if showStatus {
		m.setStatus("Selected Future")
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) setStatus(status string) {
	if m.command == nil {
		return
	}
	m.command.SetStatus(status)
	m.statusText = status
	m.statusClearActive = false
	trim := strings.TrimSpace(status)
	if trim == "" || strings.EqualFold(trim, "ready") {
		m.statusClearPending = false
		return
	}
	m.statusClearPending = true
	m.statusClearToken++
}

func (m *Model) scheduleStatusClear() tea.Cmd {
	if !m.statusClearPending || m.statusClearActive || strings.TrimSpace(m.statusText) == "" {
		return nil
	}
	m.statusClearPending = false
	m.statusClearActive = true
	token := m.statusClearToken
	return tea.Tick(statusClearTimeout, func(time.Time) tea.Msg {
		return statusClearMsg{token: token}
	})
}

func (m *Model) clearStatus() {
	m.statusClearActive = false
	m.statusClearPending = false
	m.statusText = "Ready"
	if m.command != nil {
		m.command.SetStatus("Ready")
	}
}

func (m *Model) postInteractionStatus(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg:
		return m.scheduleStatusClear()
	}
	return nil
}

func (m *Model) showReportOverlay(arg string) (tea.Cmd, string) {
	if m.command == nil {
		return nil, "noop"
	}
	if m.overlayPane == nil {
		m.overlayPane = overlaypane.New(m.width, maxInt(1, m.height-1))
	}
	if m.service == nil {
		m.setStatus("Report unavailable: service offline")
		return nil, "error"
	}
	if m.reportVisible {
		return m.closeReportOverlay(), "closed"
	}
	dur, label, err := m.parseReportWindow(arg)
	if err != nil {
		m.setStatus("Report: " + err.Error())
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

func (m *Model) showMigrateOverlay(arg string) (tea.Cmd, string) {
	if m.command == nil {
		return nil, "noop"
	}
	if m.overlayPane == nil {
		m.overlayPane = overlaypane.New(m.width, maxInt(1, m.height-1))
	}
	if m.service == nil {
		m.setStatus("Migration unavailable: service offline")
		return nil, "error"
	}
	if m.migrateVisible {
		return m.closeMigrateOverlay(), "closed"
	}
	now := m.today
	if now.IsZero() {
		now = time.Now()
	}
	window, err := resolveMigrationWindow(now, arg)
	if err != nil {
		m.setStatus("Migration: " + err.Error())
		return nil, "error"
	}
	ctx := context.Background()
	metas, err := m.service.CollectionsMeta(ctx, "")
	if err != nil {
		m.setStatus("Migration unavailable: " + err.Error())
		return nil, "error"
	}
	parsedRoots := viewmodel.BuildTree(metas)
	futureRoots := futureCollectionsFromMetas(metas, now)
	targetRoots := filterMoveCollections(parsedRoots)
	targetRoots = appendNewCollectionOption(targetRoots)
	targetRoots = includeNextMonthCollection(targetRoots, now)
	candidates, err := m.service.MigrationCandidates(ctx, window.Since, window.Until)
	if err != nil {
		m.setStatus("Migration unavailable: " + err.Error())
		return nil, "error"
	}
	data := buildMigrationData(now, candidates, parsedRoots)
	futureNav := collectionnav.NewModel(futureRoots)
	futureNav.SetBlurOnSelect(false)
	targetNav := collectionnav.NewModel(targetRoots)
	targetNav.SetBlurOnSelect(false)
	overlay := newMigrationOverlay(data, window, futureNav, targetNav, m.dump)
	overlay.SetSize(m.width, maxInt(1, m.height-1))

	if data.IsEmpty() && m.command != nil {
		m.setStatus("Migration inbox empty")
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
	if m.migrateVisible {
		if cmd := m.closeMigrateOverlay(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	placement := command.OverlayPlacement{Fullscreen: true}
	if cmd := m.overlayPane.SetOverlay(overlay, placement); cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.migrateOverlay = overlay
	m.migrateVisible = true
	m.migrateWindow = window
	_ = m.dropFocusKind(focusKindCommand)
	m.pushFocus(focusTarget{kind: focusKindOverlay, overlay: overlayKindMigrate})
	cmds = append(cmds, m.blurJournalPanes()...)
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
		m.setStatus("Help overlay closed")
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
		m.setStatus("Add task overlay opened for " + label)
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
		m.setStatus("Add task overlay closed")
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
		m.setStatus("Bullet detail overlay closed")
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
			m.setStatus(status)
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
		m.setStatus(status)
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
			m.setStatus("Report overlay closed")
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
	if m.migrateVisible {
		return m.closeMigrateOverlay()
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
