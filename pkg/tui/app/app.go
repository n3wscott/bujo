package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/davecgh/go-spew/spew"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/collection"
	viewmodel "tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/store"
	cachepkg "tableflip.dev/bujo/pkg/tui/cache"
	"tableflip.dev/bujo/pkg/tui/clock"
	collectiondetail2 "tableflip.dev/bujo/pkg/tui/components/collectiondetail2"
	collectionnav "tableflip.dev/bujo/pkg/tui/components/collectionnav"
	"tableflip.dev/bujo/pkg/tui/components/command"
	"tableflip.dev/bujo/pkg/tui/components/eventviewer"
	journalcomponent "tableflip.dev/bujo/pkg/tui/components/journal"
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
	service JournalService
	clock   clock.Clock

	width  int
	height int

	command      *command.Model
	overlayStack *overlayStack
	router       *pageRouter

	debugEnabled bool
	eventViewer  *eventviewer.Model

	cachePath     string
	dataSource    string
	report        *reportOverlay
	reportVisible bool

	dump io.Writer

	helpVisible          bool
	helpReturn           journalcomponent.FocusPane
	helpHadFocus         bool
	addVisible           bool
	addOverlay           *addtaskOverlay
	detailVisible        bool
	detailOverlay        *bulletdetailOverlay
	detailLoadID         string
	moveVisible          bool
	moveOverlay          *movebulletOverlay
	moveLoadID           string
	moveBulletID         string
	moveCollectionID     string
	moveFutureOnly       bool
	newCollectionVisible bool
	newCollectionOverlay *newCollectionOverlay
	migrateVisible       bool
	migrateOverlay       *migrationOverlay
	migrateWindow        migrationWindow

	statusText         string
	statusEpoch        int64
	statusSetEpoch     int64
	statusClearPending bool
	statusClearActive  bool
	statusClearToken   int64

	commandActive bool
	commandReturn journalcomponent.FocusPane

	journalNav     *collectionnav.Model
	journalDetail  journalcomponent.DetailPane
	journalCache   *cachepkg.Cache
	loadingJournal bool
	journalError   error

	detailMode collectiondetail2.Mode

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
	overlayKindNewCollection
	overlayKindMigrate
)

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

// Options configure the root TUI model.
type Options struct {
	Service JournalService
	Clock   clock.Clock
}

// New constructs a root model with the provided service.
func New(service JournalService) *Model {
	return NewWithOptions(Options{Service: service})
}

// NewWithOptions constructs a root model with explicit configuration.
func NewWithOptions(opts Options) *Model {
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
		{Name: "details", Description: "Switch detail mode (continuous/focused)"},
		{Name: "help", Description: "Show command tips"},
		{Name: "lock", Description: "Lock the selected task"},
		{Name: "unlock", Description: "Unlock the selected task"},
		{Name: "quit", Description: "Exit bujo"},
		{Name: "report", Description: "Show completed entries report"},
		{Name: "debug", Description: "Toggle debug event viewer"},
		{Name: "migrate", Description: "Review and migrate open tasks"},
	})
	ctx, cancel := context.WithCancel(context.Background())
	clk := opts.Clock
	if clk == nil {
		clk = clock.RealClock{}
	}
	return &Model{
		service:      opts.Service,
		clock:        clk,
		command:      cmd,
		overlayStack: newOverlayStack(1, 1),
		router:       newPageRouter(),
		cachePath:    cachePath,
		dataSource:   dataSource,
		ctx:          ctx,
		cancel:       cancel,
		today:        startOfDay(clk.Now()),
		detailMode:   collectiondetail2.ModeContinuous,
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

// Update routes Bubble Tea messages to composed components and returns the updated model with any commands to execute.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.noteEvent(msg)
	m.statusEpoch++

	var cmds []tea.Cmd
	skipCommandUpdate := false
	skipJournalKey := false

	if m.dump != nil {
		_, _ = fmt.Fprintf(m.dump, "%s ", time.Now().Format("2006-01-02T15:04:05"))
		spew.Fdump(m.dump, msg)
	}

	switch v := msg.(type) {
	case tea.FocusMsg:
		m.refreshToday(m.now())
	case tea.BlurMsg:
		// no-op (we'll rely on periodic checks)
	case dayCheckMsg:
		m.refreshToday(m.now())
		if cmd := m.scheduleDayCheck(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case events.ChildMsg:
		if m.journalCache != nil && v.From == m.journalCache.ComponentID() {
			if cmd := cacheListenCmd(m.journalCache); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if v.Msg != nil {
			nextModel, innerCmd := m.Update(v.Msg)
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
			if m.overlayStack != nil && m.overlayStack.HasOverlay() {
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
		if next, handled := m.handleCommandSubmit(v); handled {
			cmds = append(cmds, next...)
		}
	case events.CommandCancelMsg:
		if next, handled := m.handleCommandCancel(v); handled {
			cmds = append(cmds, next...)
		}
	case events.CommandChangeMsg:
		if next, handled := m.handleCommandChange(v); handled {
			cmds = append(cmds, next...)
		}
	case tea.QuitMsg:
		m.stopWatch()
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
	case events.AddTaskRequestMsg:
		if m.migrateVisible || m.moveVisible || m.detailVisible {
			break
		}
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
	case moveCreateCollectionMsg:
		if cmd := m.handleMoveCreateCollection(v.Name); cmd != nil {
			cmds = append(cmds, cmd)
		}
		skipJournalKey = true
		skipCommandUpdate = true
	case moveCreateCollectionCancelledMsg:
		if m.command != nil {
			m.setStatus("New collection creation cancelled")
		}
		if m.moveOverlay != nil {
			if cmd := m.moveOverlay.FocusNav(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		skipJournalKey = true
		skipCommandUpdate = true
	case newCollectionCreateMsg:
		if cmd := m.handleNewCollectionCreate(v.Name); cmd != nil {
			cmds = append(cmds, cmd)
		}
		skipJournalKey = true
		skipCommandUpdate = true
	case newCollectionCancelledMsg:
		if cmd := m.closeNewCollectionOverlayWithStatus("New collection creation cancelled"); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if cmd := m.journalFocusCmd(journalcomponent.FocusNav); cmd != nil {
			cmds = append(cmds, cmd)
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
			break
		}
		if m.journalNav != nil && v.Component == m.journalNav.ID() && strings.EqualFold(strings.TrimSpace(v.Collection.ID), newCollectionOptionID) {
			if cmd := m.startNewCollectionPrompt(); cmd != nil {
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
			Clock:     m.clock,
		})
		cache.SetCollections(snap.Metas)
		cache.SetSections(snap.Sections)
		navCollections := append([]*viewmodel.ParsedCollection(nil), snap.Collections...)
		navCollections = appendNewCollectionOption(navCollections)
		nav := collectionnav.NewModel(navCollections)
		if !m.today.IsZero() {
			nav.SetNow(m.now())
		}
		nav.SetID(events.ComponentID("MainNav"))
		detail := collectiondetail2.NewModel(snap.Sections)
		detail.SetID(events.ComponentID("DetailPane"))
		detail.SetSourceNav(nav.ID())
		detail.SetMode(m.detailMode)
		if m.dump != nil {
			detail.SetDebugWriter(m.dump)
		}
		journal := journalcomponent.NewModel(nav, detail, cache)
		journal.SetID(events.ComponentID("JournalPane"))
		m.setJournal(journal)
		if cmd := m.journalFocusCmd(journalcomponent.FocusNav); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if m.command != nil {
			m.command.Blur()
		}
		m.journalCache = cache
		m.journalNav = nav
		m.journalDetail = detail
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

	if m.overlayBlocksCommand() {
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

	if m.overlayStack != nil {
		closed, cmd := m.overlayStack.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if closed != overlayKindNone {
			if cmd := m.closeOverlay(closed); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	if m.router != nil {
		skipUpdate := false
		if _, isKey := msg.(tea.KeyMsg); isKey {
			if skipJournalKey || m.overlayBlocksJournal() {
				skipUpdate = true
			}
		}
		if !skipUpdate {
			next, cmd := m.router.Update(msg)
			if r, ok := next.(*pageRouter); ok {
				m.router = r
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
	journal := m.journal()
	if journal == nil {
		m.setStatus("Lock unavailable: journal cache offline")
		return nil
	}
	section, bullet, ok := journal.CurrentSelection()
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
	journal := m.journal()
	if journal == nil {
		m.setStatus("Unlock unavailable: journal cache offline")
		return nil
	}
	section, bullet, ok := journal.CurrentSelection()
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
	now := m.now()
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
	if cmd := m.journalFocusCmd(journalcomponent.FocusDetail); cmd != nil {
		cmds = append(cmds, cmd)
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
	if cmd := m.journalFocusCmd(journalcomponent.FocusDetail); cmd != nil {
		cmds = append(cmds, cmd)
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
	case events.JournalFocusMsg:
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
