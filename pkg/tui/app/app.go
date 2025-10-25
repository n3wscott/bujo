// Package teaui hosts the Bubble Tea program for the bujo TUI.
package teaui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/store"
	"tableflip.dev/bujo/pkg/timeutil"
	"tableflip.dev/bujo/pkg/tui/components/bottombar"
	"tableflip.dev/bujo/pkg/tui/components/detail"
	"tableflip.dev/bujo/pkg/tui/components/index"
	"tableflip.dev/bujo/pkg/tui/components/panel"
	"tableflip.dev/bujo/pkg/tui/theme"
	"tableflip.dev/bujo/pkg/tui/uiutil"
	migrationview "tableflip.dev/bujo/pkg/tui/views/migration"
	reportview "tableflip.dev/bujo/pkg/tui/views/report"
	wizardview "tableflip.dev/bujo/pkg/tui/views/wizard"
)

// Model states and actions
type mode int

const (
	modeNormal mode = iota
	modeInsert
	modeCommand
	modeCollectionWizard
	modeHelp
	modeBulletSelect
	modePanel
	modeConfirm
	modeParentSelect
	modeReport
	modeMigration
)

type action int

const (
	actionNone action = iota
	actionAdd
	actionEdit
	actionMove
)

type menuSection int

const (
	menuSectionBullet menuSection = iota
	menuSectionSignifier
)

type collectionDescriptor struct {
	id       string
	name     string
	resolved string
}

type confirmAction int

const (
	confirmNone confirmAction = iota
	confirmDeleteEntry
	confirmDeleteCollection
)

type commandContext int

const (
	commandContextGlobal commandContext = iota
	commandContextMove
)

var (
	errServiceUnavailable = errors.New("service unavailable")
	errInvalidCollection  = errors.New("invalid collection name")
)

type parentCandidate struct {
	ID    string
	Label string
}

type parentSelectState struct {
	active     bool
	childID    string
	collection string
	candidates []parentCandidate
	index      int
}

var commandDefinitions = []bottombar.CommandOption{
	{Name: "q", Description: "Quit application"},
	{Name: "quit", Description: "Quit application"},
	{Name: "exit", Description: "Quit application"},
	{Name: "today", Description: "Jump to Today collection"},
	{Name: "future", Description: "Jump to Future collection"},
	{Name: "report", Description: "Generate completion report"},
	{Name: "migrate", Description: "Review open tasks in a window"},
	{Name: "new-collection", Description: "Create a collection with guided wizard"},
	{Name: "type", Description: "Set collection type"},
	{Name: "help", Description: "Show help guide"},
	{Name: "mkdir", Description: "Create collection (supports hierarchy)"},
	{Name: "show-hidden", Description: "Toggle moved originals visibility"},
	{Name: "lock", Description: "Lock selected entry"},
	{Name: "unlock", Description: "Unlock selected entry"},
	{Name: "delete", Description: "Delete selected entry"},
	{Name: "delete-collection", Description: "Remove a collection (confirmation)"},
}

// Model contains UI state
type Model struct {
	svc        *app.Service
	ctx        context.Context
	cancel     context.CancelFunc
	mode       mode
	resumeMode mode
	action     action

	focus int // 0: collections, 1: entries

	colList list.Model

	input textinput.Model

	pendingBullet     glyph.Bullet
	bulletOptions     []glyph.Bullet
	bulletIndex       int
	bulletTargetID    string
	awaitingDD        bool
	lastDTime         time.Time
	signifierOptions  []glyph.Signifier
	bulletMenuOptions []bulletMenuOption
	bulletMenuFocus   menuSection

	termWidth               int
	termHeight              int
	verticalReserve         int
	overlayReserve          int
	indexState              *index.State
	pendingResolved         string
	detailWidth             int
	detailHeight            int
	detailState             *detail.State
	entriesCache            map[string][]*entry.Entry
	entriesMu               sync.RWMutex
	detailOrder             []collectionDescriptor
	panelModel              panel.Model
	panelEntryID            string
	panelCollection         string
	confirmAction           confirmAction
	confirmTargetID         string
	confirmTargetCollection string
	watchCh                 <-chan store.Event
	watchCancel             context.CancelFunc
	detailRevealTarget      string
	pendingChildParent      string
	parentSelect            parentSelectState
	pendingAddCollection    string
	showHiddenMoved         bool
	pendingSweep            bool

	focusDel list.DefaultDelegate
	blurDel  list.DefaultDelegate

	bottom    bottombar.Model
	helpLines []string

	commandSelectActive  bool
	commandOriginalInput string

	commandContext          commandContext
	moveCollections         []string
	moveTargetID            string
	pendingCreateCollection string
	pendingCreateType       collection.Type

	wizard *wizardview.Model

	migration *migrationview.Model

	theme theme.Theme

	report *reportview.Model
}

type bulletMenuOption struct {
	section   menuSection
	bullet    glyph.Bullet
	signifier glyph.Signifier
}

// New creates a new UI model backed by the Service.
func New(svc *app.Service) *Model {
	dFocus := list.NewDefaultDelegate()
	dBlur := list.NewDefaultDelegate()
	// Unfocused list should not visually highlight the selected item
	dBlur.Styles.SelectedTitle = dBlur.Styles.NormalTitle
	dBlur.Styles.SelectedDesc = dBlur.Styles.NormalDesc
	dFocus.ShowDescription = false
	dBlur.ShowDescription = false
	dFocus.SetSpacing(0)
	dBlur.SetSpacing(0)

	l1 := list.New([]list.Item{}, dBlur, 24, 20)
	l1.SetShowHelp(false)
	l1.SetShowStatusBar(false)
	l1.SetShowTitle(false)

	ti := textinput.New()
	ti.Placeholder = "Type here"
	ti.CharLimit = 256
	ti.Focus()
	ti.Prompt = ""
	ti.VirtualCursor = true
	ti.Styles.Cursor.Color = lipgloss.Color("212")
	ti.Styles.Cursor.Shape = tea.CursorBlock
	ti.Styles.Cursor.Blink = true

	bulletOpts := []glyph.Bullet{glyph.Task, glyph.Note, glyph.Event, glyph.Completed, glyph.Irrelevant}
	signifierOpts := []glyph.Signifier{glyph.None, glyph.Priority, glyph.Inspiration, glyph.Investigation}

	th := theme.Default()
	bottom := bottombar.New(th.Footer)
	report := reportview.New(th, relativeTime)
	wizard := wizardview.New(th)
	migration := migrationview.New(th, relativeTime)
	ctx, cancel := context.WithCancel(context.Background())

	m := &Model{
		svc:              svc,
		ctx:              ctx,
		cancel:           cancel,
		mode:             modeNormal,
		action:           actionNone,
		focus:            1,
		colList:          l1,
		input:            ti,
		pendingBullet:    glyph.Task,
		focusDel:         dFocus,
		blurDel:          dBlur,
		bulletOptions:    bulletOpts,
		signifierOptions: signifierOpts,
		indexState:       index.NewState(),
		bottom:           bottom,
		resumeMode:       modeNormal,
		detailState:      detail.NewState(),
		entriesCache:     make(map[string][]*entry.Entry),
		panelModel:       panel.New(th.Panel),
		bulletMenuFocus:  menuSectionBullet,
		pendingSweep:     true,
		theme:            th,
		report:           report,
		wizard:           wizard,
		migration:        migration,
	}
	m.bulletIndex = m.findBulletIndex(m.pendingBullet)
	m.bottom.SetPendingBullet(m.pendingBullet)
	m.bottom.SetMode(bottombar.ModeNormal)
	m.updateBottomContext()
	m.applyReserve()
	m.updateFocusHeaders()
	m.pendingResolved = todayResolvedCollection()
	return m
}

// Init loads initial data
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.refreshAll(), startWatchCmd(m.ctx, m.svc))
}

func (m *Model) refreshAll() tea.Cmd {
	m.clearEntriesCache()
	return m.loadCollections()
}

func (m *Model) clearEntriesCache() {
	m.entriesMu.Lock()
	m.entriesCache = make(map[string][]*entry.Entry)
	m.entriesMu.Unlock()
}

func (m *Model) invalidateCollectionCache(collection string) {
	if collection == "" {
		m.clearEntriesCache()
		return
	}
	m.entriesMu.Lock()
	delete(m.entriesCache, collection)
	m.entriesMu.Unlock()
}

func (m *Model) loadCollections() tea.Cmd {
	current := m.currentResolvedCollection()
	now := time.Now()
	return func() tea.Msg {
		metas, err := m.svc.CollectionsMeta(m.ctx, "")
		if err != nil {
			return errMsg{err}
		}
		items := m.buildCollectionItems(metas, current, now)
		return collectionsLoadedMsg{items: items}
	}
}

func (m *Model) selectedCollection() string {
	if len(m.colList.Items()) == 0 {
		return ""
	}
	sel := m.colList.SelectedItem()
	if sel == nil {
		return ""
	}
	switch v := sel.(type) {
	case index.CollectionItem:
		if v.Resolved == index.TrackingGroupKey {
			return ""
		}
		if v.Resolved != "" {
			return v.Resolved
		}
		return v.Name
	case *index.CalendarRowItem:
		state := m.indexState.Months[v.Month]
		if state == nil {
			return ""
		}
		day := m.indexState.Selection[v.Month]
		if day == 0 || !index.ContainsDay(v.Days, day) {
			day = index.FirstNonZero(v.Days)
		}
		if day == 0 {
			return ""
		}
		return index.FormatDayPath(state.MonthTime, day)
	case *index.CalendarHeaderItem:
		return v.Month
	default:
		return ""
	}
}

func (m *Model) activeCollectionCandidate() string {
	if m.detailState != nil {
		if id := m.detailState.ActiveCollectionID(); id != "" {
			return id
		}
	}
	return m.selectedCollection()
}

func (m *Model) loadDetailSections() tea.Cmd {
	return m.loadDetailSectionsWithFocus("", "")
}

func (m *Model) loadDetailSectionsWithFocus(preferredCollection, preferredEntry string) tea.Cmd {
	order := m.buildDetailOrder()

	focus := preferredCollection
	if focus == "" && m.detailState != nil {
		focus = m.detailState.ActiveCollectionID()
	}
	if focus == "" {
		focus = m.selectedCollection()
	}

	focusEntry := preferredEntry
	if focusEntry == "" && m.detailState != nil {
		focusEntry = m.detailState.ActiveEntryID()
	}

	return func() tea.Msg {
		seen := make(map[string]bool)
		orderIndex := make(map[string]int, len(order))
		for i, desc := range order {
			if desc.id != "" {
				orderIndex[desc.id] = i
			}
		}

		sections := make([]detail.Section, 0, len(order))
		sectionOrder := make([]int, 0, len(order))
		visibleSet := make(map[string]bool, len(order))

		addSection := func(desc collectionDescriptor, entries []*entry.Entry, force bool) bool {
			visible, hasVisible := m.filterEntriesForDisplay(entries)
			if !hasVisible && !force {
				if desc.id != "" {
					visibleSet[desc.id] = false
					seen[desc.id] = true
				}
				return false
			}
			if desc.id != "" {
				visibleSet[desc.id] = true
			}
			name := desc.name
			if strings.Contains(desc.id, "/") {
				name = uiutil.FormattedCollectionName(desc.id)
			} else if name == "" {
				name = uiutil.FriendlyCollectionName(desc.id)
			}
			sections = append(sections, detail.Section{
				CollectionID:   desc.id,
				CollectionName: name,
				ResolvedName:   desc.resolved,
				Entries:        visible,
			})
			if idx, ok := orderIndex[desc.id]; ok {
				sectionOrder = append(sectionOrder, idx)
			} else {
				sectionOrder = append(sectionOrder, len(order)+len(sectionOrder))
			}
			seen[desc.id] = true
			return true
		}

		for _, desc := range order {
			if desc.id == "" || seen[desc.id] {
				continue
			}
			entries, err := m.entriesForCollection(desc.id)
			if err != nil {
				return errMsg{err}
			}
			force := desc.id == focus
			if _, hasVisible := m.filterEntriesForDisplay(entries); !hasVisible && !force {
				continue
			}
			addSection(desc, entries, force)
		}

		if focus != "" && !seen[focus] {
			entries, err := m.entriesForCollection(focus)
			if err != nil {
				return errMsg{err}
			}
			addSection(m.descriptorForCollection(focus), entries, true)
		}

		if len(sections) == 0 && focus != "" {
			addSection(m.descriptorForCollection(focus), nil, true)
		}

		if len(sections) > 1 {
			type pair struct {
				sec detail.Section
				idx int
			}
			pairs := make([]pair, len(sections))
			for i := range sections {
				pairs[i] = pair{sec: sections[i], idx: sectionOrder[i]}
			}
			sort.SliceStable(pairs, func(i, j int) bool {
				if pairs[i].idx == pairs[j].idx {
					return strings.Compare(pairs[i].sec.CollectionName, pairs[j].sec.CollectionName) < 0
				}
				return pairs[i].idx < pairs[j].idx
			})
			for i := range pairs {
				sections[i] = pairs[i].sec
				sectionOrder[i] = pairs[i].idx
			}
		}

		if len(sections) == 0 {
			return detailSectionsLoadedMsg{sections: sections, activeCollection: focus, activeEntry: "", visible: visibleSet}
		}

		if focusEntry != "" {
			found := false
			for _, sec := range sections {
				if sec.CollectionID != focus {
					continue
				}
				for _, ent := range sec.Entries {
					if ent.ID == focusEntry {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				focusEntry = ""
			}
		}

		m.detailOrder = make([]collectionDescriptor, len(sections))
		for i, sec := range sections {
			m.detailOrder[i] = collectionDescriptor{id: sec.CollectionID, name: sec.CollectionName, resolved: sec.ResolvedName}
		}

		return detailSectionsLoadedMsg{sections: sections, activeCollection: focus, activeEntry: focusEntry, visible: visibleSet}
	}
}

// messages
type errMsg struct{ err error }
type collectionsLoadedMsg struct{ items []list.Item }
type detailSectionsLoadedMsg struct {
	sections         []detail.Section
	activeCollection string
	activeEntry      string
	visible          map[string]bool
}

type detailFocusChangedMsg struct {
	Collection string
	Entry      string
	FromDetail bool
}

func detailFocusChangedCmd(collection, entry string, fromDetail bool) tea.Cmd {
	if collection == "" {
		return nil
	}
	return func() tea.Msg {
		return detailFocusChangedMsg{
			Collection: collection,
			Entry:      entry,
			FromDetail: fromDetail,
		}
	}
}

type watchStartedMsg struct {
	ch     <-chan store.Event
	cancel context.CancelFunc
	err    error
}

type watchEventMsg struct {
	event store.Event
}

type watchStoppedMsg struct{}

func startWatchCmd(parent context.Context, svc *app.Service) tea.Cmd {
	if svc == nil {
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

func (m *Model) handleWatchEvent(ev store.Event, cmds *[]tea.Cmd) {
	switch ev.Type {
	case store.EventCollectionChanged:
		m.invalidateCollectionCache(ev.Collection)
		*cmds = append(*cmds, m.loadCollections(), m.loadDetailSectionsWithFocus(ev.Collection, ""))
	case store.EventCollectionsInvalidated:
		m.clearEntriesCache()
		*cmds = append(*cmds, m.loadCollections(), m.loadDetailSections())
	default:
		m.clearEntriesCache()
		*cmds = append(*cmds, m.loadCollections(), m.loadDetailSections())
	}
}

func (m *Model) handleKeyPress(msg tea.KeyPressMsg, cmds *[]tea.Cmd) bool {
	switch m.mode {
	case modeHelp:
		return m.handleHelpKey(msg)
	case modeBulletSelect:
		return m.handleBulletSelectKey(msg, cmds)
	case modeInsert:
		return m.handleInsertKey(msg, cmds)
	case modeCommand:
		return m.handleCommandKey(msg, cmds)
	case modeCollectionWizard:
		return m.handleCollectionWizardKey(msg, cmds)
	case modePanel:
		return m.handlePanelKey(msg, cmds)
	case modeConfirm:
		return m.handleConfirmKey(msg, cmds)
	case modeParentSelect:
		return m.handleParentSelectKey(msg, cmds)
	case modeReport:
		return m.handleReportKey(msg)
	case modeMigration:
		return m.handleMigrationKey(msg, cmds)
	case modeNormal:
		return m.handleNormalKey(msg, cmds)
	default:
		return true
	}
}

func (m *Model) handleHelpKey(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "q", "esc", "?":
		m.helpLines = nil
		m.setMode(modeNormal)
		m.setOverlayReserve(0)
		return true
	default:
		return false
	}
}

func (m *Model) handlePanelKey(msg tea.KeyPressMsg, cmds *[]tea.Cmd) bool {
	switch msg.String() {
	case "esc", "enter":
		m.closePanel()
		return true
	case "e":
		if it := m.currentEntry(); it != nil {
			m.closePanel()
			m.beginEdit(it, cmds)
		}
		return true
	case "b":
		if it := m.currentEntry(); it != nil {
			m.closePanel()
			m.enterBulletSelect(it.ID, menuSectionBullet)
		}
		return true
	case "v":
		if it := m.currentEntry(); it != nil {
			m.closePanel()
			m.enterBulletSelect(it.ID, menuSectionSignifier)
		} else {
			m.setStatus("No entry selected")
		}
		return true
	case "?":
		m.closePanel()
		m.showHelpPanel()
		return true
	default:
		return false
	}
}

func (m *Model) handleConfirmKey(msg tea.KeyPressMsg, cmds *[]tea.Cmd) bool {
	switch msg.String() {
	case "enter":
		input := strings.TrimSpace(strings.ToLower(m.input.Value()))
		if input == "yes" {
			switch m.confirmAction {
			case confirmDeleteEntry:
				m.applyDelete(cmds, m.confirmTargetID)
			case confirmDeleteCollection:
				m.applyDeleteCollection(cmds, m.confirmTargetCollection)
			}
		} else {
			m.setStatus("Type yes to confirm")
		}
		return true
	case "esc", "q":
		action := m.confirmAction
		m.cancelConfirm()
		switch action {
		case confirmDeleteCollection:
			m.setStatus("Collection delete cancelled")
		case confirmDeleteEntry:
			m.setStatus("Delete cancelled")
		default:
			m.setStatus("Cancelled")
		}
		return true
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if cmd != nil {
			*cmds = append(*cmds, cmd)
		}
		return false
	}
}

func (m *Model) handleBulletSelectKey(msg tea.KeyPressMsg, cmds *[]tea.Cmd) bool {
	if len(m.bulletMenuOptions) == 0 {
		m.exitBulletSelect(cmds)
		return true
	}
	switch msg.String() {
	case "esc", "q":
		m.exitBulletSelect(cmds)
		return true
	case "enter":
		opt := m.bulletMenuOptions[m.bulletIndex]
		switch opt.section {
		case menuSectionBullet:
			chosen := opt.bullet
			if m.bulletTargetID == "" {
				m.pendingBullet = chosen
				m.bottom.SetPendingBullet(m.pendingBullet)
				m.setStatus(fmt.Sprintf("Default bullet set to %s", chosen.Glyph().Meaning))
			} else {
				m.applySetBullet(cmds, m.bulletTargetID, chosen)
			}
			m.exitBulletSelect(cmds)
			return true
		case menuSectionSignifier:
			if m.bulletTargetID == "" {
				m.setStatus("Signifiers apply to existing entries")
				return true
			}
			m.applySetSignifier(cmds, m.bulletTargetID, opt.signifier)
			m.exitBulletSelect(cmds)
			return true
		}
	case "up", "k":
		if m.bulletIndex > 0 {
			m.bulletIndex--
		} else {
			m.bulletIndex = len(m.bulletMenuOptions) - 1
		}
	case "down", "j":
		if m.bulletIndex < len(m.bulletMenuOptions)-1 {
			m.bulletIndex++
		} else {
			m.bulletIndex = 0
		}
	case "tab", "shift+tab":
		var next menuSection
		if m.bulletMenuFocus == menuSectionBullet {
			next = menuSectionSignifier
		} else {
			next = menuSectionBullet
		}
		if idx := m.menuFirstIndex(next); idx >= 0 {
			m.bulletIndex = idx
			m.bulletMenuFocus = next
		}
		return true
	case "left", "h":
		if idx := m.menuFirstIndex(menuSectionBullet); idx >= 0 {
			m.bulletIndex = idx
			m.bulletMenuFocus = menuSectionBullet
		}
		return true
	case "right", "l":
		if idx := m.menuFirstIndex(menuSectionSignifier); idx >= 0 {
			m.bulletIndex = idx
			m.bulletMenuFocus = menuSectionSignifier
		}
		return true
	}
	if len(m.bulletMenuOptions) > 0 {
		m.bulletMenuFocus = m.bulletMenuOptions[m.bulletIndex].section
	}
	return false
}

func (m *Model) handleInsertKey(msg tea.KeyPressMsg, cmds *[]tea.Cmd) bool {
	switch msg.String() {
	case "enter":
		input := strings.TrimSpace(m.input.Value())
		m.submitInsert(input, cmds)
		return true
	case "esc":
		m.cancelInsert()
		return true
	case "ctrl+b":
		m.enterBulletSelect("", menuSectionBullet)
		return true
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if cmd != nil {
			*cmds = append(*cmds, cmd)
		}
		return false
	}
}

func (m *Model) handleCommandKey(msg tea.KeyPressMsg, cmds *[]tea.Cmd) bool {
	switch msg.String() {
	case "enter":
		if m.commandContext == commandContextMove {
			if m.pendingCreateCollection != "" {
				target := m.pendingCreateCollection
				entryID := m.moveTargetID
				if entryID == "" {
					m.exitMoveSelector(true)
					m.pendingCreateCollection = ""
					m.pendingCreateType = ""
					m.setStatus("Move cancelled")
					return true
				}
				canonical, typ, err := m.ensureCollectionPath(target)
				if err != nil {
					if errors.Is(err, errServiceUnavailable) {
						m.exitMoveSelector(true)
					}
					m.pendingCreateCollection = ""
					m.pendingCreateType = ""
					m.setStatus("Move: " + err.Error())
					return true
				}
				m.pendingCreateCollection = ""
				m.pendingCreateType = ""
				m.applyMove(cmds, entryID, canonical)
				m.setStatus(fmt.Sprintf("Move: created %s as %s", canonical, strings.ToLower(string(typ))))
				m.exitMoveSelector(false)
				return true
			}
			if err := m.finishMoveSelection(cmds); err != nil {
				m.setStatus("Move: " + err.Error())
			}
			return true
		}
		input := strings.TrimSpace(m.input.Value())
		m.executeCommand(input, cmds)
		return true
	case "esc":
		if m.commandContext == commandContextMove {
			if m.commandSelectActive {
				m.commandSelectActive = false
				m.bottom.ClearSuggestion()
				m.input.SetValue(m.commandOriginalInput)
				m.input.CursorEnd()
				m.updateMoveSuggestions(m.input.Value())
				m.applyReserve()
				m.setStatus("Move selection cleared")
				return true
			}
			m.exitMoveSelector(true)
			return true
		}
		if m.commandSelectActive {
			m.commandSelectActive = false
			m.bottom.ClearSuggestion()
			m.input.SetValue(m.commandOriginalInput)
			m.input.CursorEnd()
			m.bottom.UpdateCommandPreview(m.input.Value(), m.input.View())
			m.applyReserve()
			m.setStatus("Command selection cleared")
			return true
		}
		m.commandOriginalInput = ""
		m.bottom.ClearSuggestion()
		m.setMode(modeNormal)
		m.input.Reset()
		m.input.Blur()
		m.bottom.UpdateCommandInput("", "")
		m.setStatus("Command cancelled")
		m.setOverlayReserve(0)
		return true
	case "tab", "down":
		if m.commandContext == commandContextMove {
			if opt, ok := m.bottom.StepSuggestion(1); ok {
				if !m.commandSelectActive {
					m.commandSelectActive = true
					m.commandOriginalInput = m.input.Value()
				}
				m.input.SetValue(opt.Name)
				m.input.CursorEnd()
				m.bottom.UpdateCommandPreview(m.input.Value(), m.input.View())
				m.applyReserve()
				m.pendingCreateCollection = ""
				m.pendingCreateType = ""
			}
			return true
		}
		if opt, ok := m.bottom.StepSuggestion(1); ok {
			if !m.commandSelectActive {
				m.commandSelectActive = true
				m.commandOriginalInput = m.input.Value()
			}
			m.input.SetValue(opt.Name)
			m.input.CursorEnd()
			m.bottom.UpdateCommandPreview(m.input.Value(), m.input.View())
			m.applyReserve()
		}
		return true
	case "shift+tab", "up":
		if m.commandContext == commandContextMove {
			if opt, ok := m.bottom.StepSuggestion(-1); ok {
				if !m.commandSelectActive {
					m.commandSelectActive = true
					m.commandOriginalInput = m.input.Value()
				}
				m.input.SetValue(opt.Name)
				m.input.CursorEnd()
				m.bottom.UpdateCommandPreview(m.input.Value(), m.input.View())
				m.applyReserve()
				m.pendingCreateCollection = ""
				m.pendingCreateType = ""
			}
			return true
		}
		if opt, ok := m.bottom.StepSuggestion(-1); ok {
			if !m.commandSelectActive {
				m.commandSelectActive = true
				m.commandOriginalInput = m.input.Value()
			}
			m.input.SetValue(opt.Name)
			m.input.CursorEnd()
			m.bottom.UpdateCommandPreview(m.input.Value(), m.input.View())
			m.applyReserve()
		}
		return true
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if cmd != nil {
			*cmds = append(*cmds, cmd)
		}
		if m.commandContext == commandContextMove {
			m.commandSelectActive = false
			m.commandOriginalInput = ""
			m.updateMoveSuggestions(m.input.Value())
			m.pendingCreateCollection = ""
			m.pendingCreateType = ""
		} else {
			m.bottom.UpdateCommandInput(m.input.Value(), m.input.View())
			m.commandSelectActive = false
			m.commandOriginalInput = ""
		}
		m.applyReserve()
		return false
	}
}

func (m *Model) handleCollectionWizardKey(msg tea.KeyPressMsg, cmds *[]tea.Cmd) bool {
	if m.wizard == nil || !m.wizard.Active {
		return false
	}
	key := msg.String()
	switch key {
	case "esc":
		m.cancelCollectionWizard("New collection cancelled")
		return true
	case "ctrl+b":
		if m.wizard.Step == wizardview.StepParent {
			m.cancelCollectionWizard("New collection cancelled")
		} else {
			m.previousCollectionWizardStep(cmds)
		}
		return true
	}

	switch m.wizard.Step {
	case wizardview.StepParent:
		options := m.wizard.ParentOptionsWithRoot()
		count := len(options)
		if count == 0 {
			count = 1
		}
		switch key {
		case "down", "j", "tab":
			m.wizard.ParentIndex = (m.wizard.ParentIndex + 1) % count
			m.updateCollectionWizardView()
			if len(options) > 0 {
				selection := options[m.wizard.ParentIndex]
				m.setStatus(fmt.Sprintf("New collection · parent %s", selection))
			}
			return true
		case "up", "k", "shift+tab":
			m.wizard.ParentIndex = (m.wizard.ParentIndex - 1 + count) % count
			m.updateCollectionWizardView()
			if len(options) > 0 {
				selection := options[m.wizard.ParentIndex]
				m.setStatus(fmt.Sprintf("New collection · parent %s", selection))
			}
			return true
		case "enter":
			return m.advanceCollectionWizard(cmds)
		}
	case wizardview.StepName:
		switch key {
		case "enter":
			return m.advanceCollectionWizard(cmds)
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			if cmd != nil {
				*cmds = append(*cmds, cmd)
			}
			m.bottom.UpdateCommandInput(m.input.Value(), m.input.View())
			return false
		}
	case wizardview.StepType:
		if len(m.wizard.TypeOptions) == 0 {
			m.wizard.TypeOptions = collection.AllTypes()
		}
		count := len(m.wizard.TypeOptions)
		switch key {
		case "down", "j", "tab":
			if count > 0 {
				m.wizard.TypeIndex = (m.wizard.TypeIndex + 1) % count
				m.updateCollectionWizardView()
				label := strings.ToLower(string(m.wizard.TypeOptions[m.wizard.TypeIndex]))
				m.setStatus(fmt.Sprintf("New collection · type %s (Enter next)", label))
			}
			return true
		case "up", "k", "shift+tab":
			if count > 0 {
				m.wizard.TypeIndex = (m.wizard.TypeIndex - 1 + count) % count
				m.updateCollectionWizardView()
				label := strings.ToLower(string(m.wizard.TypeOptions[m.wizard.TypeIndex]))
				m.setStatus(fmt.Sprintf("New collection · type %s (Enter next)", label))
			}
			return true
		case "enter":
			return m.advanceCollectionWizard(cmds)
		}
	case wizardview.StepConfirm:
		if key == "enter" {
			return m.advanceCollectionWizard(cmds)
		}
		return true
	}
	return false
}

func (m *Model) handleNormalKey(msg tea.KeyPressMsg, cmds *[]tea.Cmd) bool {
	key := msg.String()
	switch key {
	case ":":
		m.enterCommandMode(cmds)
		return true
	case "/":
		// prevent list filter activation; handled via command mode instead
		return true
	case "esc":
		if m.focus == 1 {
			m.focus = 0
			m.updateFocusHeaders()
			m.updateBottomContext()
			*cmds = append(*cmds, m.loadDetailSectionsWithFocus(m.selectedCollection(), ""))
			if cmd := m.syncCollectionIndicators(); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
			return true
		}
		m.setStatus("Use :q or :exit to quit")
		return true
	case "h", "left":
		if m.focus == 0 {
			if cmd := m.moveCalendarCursor(-1, 0); cmd != nil {
				*cmds = append(*cmds, cmd)
				return true
			}
		}
		m.focus = 0
		m.updateFocusHeaders()
		m.updateBottomContext()
		*cmds = append(*cmds, m.loadDetailSectionsWithFocus(m.selectedCollection(), ""))
		if cmd := m.syncCollectionIndicators(); cmd != nil {
			*cmds = append(*cmds, cmd)
		}
		return true
	case "l", "right":
		if m.focus == 0 {
			if cmd := m.moveCalendarCursor(1, 0); cmd != nil {
				*cmds = append(*cmds, cmd)
				return true
			}
		}
		m.focus = 1
		m.updateFocusHeaders()
		m.updateBottomContext()
		*cmds = append(*cmds, m.loadDetailSections())
		if cmd := m.syncCollectionIndicators(); cmd != nil {
			*cmds = append(*cmds, cmd)
		}
		return true
	case "enter":
		if m.focus == 0 {
			if cmd := m.markCalendarSelection(); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
			target := m.selectedCollection()
			if target != "" {
				m.focus = 1
				m.updateFocusHeaders()
				m.updateBottomContext()
				if cmd := m.syncCollectionIndicators(); cmd != nil {
					*cmds = append(*cmds, cmd)
				}
				m.detailRevealTarget = target
				*cmds = append(*cmds, m.loadDetailSectionsWithFocus(target, ""))
			}
			return true
		}
		if it := m.currentEntry(); it != nil {
			m.openTaskPanel(it)
		}
		return true
	case "j", "down":
		if m.focus == 0 {
			if cmd := m.moveCalendarCursor(0, 1); cmd != nil {
				*cmds = append(*cmds, cmd)
				return true
			}
			m.colList.CursorDown()
			m.ensureCollectionSelection(1)
			m.updateActiveMonthFromSelection(false, cmds)
			m.detailRevealTarget = m.selectedCollection()
			*cmds = append(*cmds, m.loadDetailSectionsWithFocus(m.selectedCollection(), ""))
			return true
		}
		if m.moveDetailCursor(1, cmds) {
			return true
		}
		return false
	case "k", "up":
		if m.focus == 0 {
			if cmd := m.moveCalendarCursor(0, -1); cmd != nil {
				*cmds = append(*cmds, cmd)
				return true
			}
			m.colList.CursorUp()
			m.ensureCollectionSelection(-1)
			m.updateActiveMonthFromSelection(false, cmds)
			m.detailRevealTarget = m.selectedCollection()
			*cmds = append(*cmds, m.loadDetailSectionsWithFocus(m.selectedCollection(), ""))
			return true
		}
		if m.moveDetailCursor(-1, cmds) {
			return true
		}
		return false
	case "g":
		if m.focus == 0 {
			m.colList.Select(0)
			if cmd := m.syncCollectionIndicators(); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
			m.updateActiveMonthFromSelection(false, cmds)
			m.detailRevealTarget = m.selectedCollection()
			*cmds = append(*cmds, m.markCalendarSelection())
			*cmds = append(*cmds, m.loadDetailSectionsWithFocus(m.selectedCollection(), ""))
		} else {
			m.detailState.ScrollToTop()
			sections := m.detailState.Sections()
			if len(sections) > 0 {
				first := sections[0].CollectionID
				m.selectCollectionByID(first, cmds)
				*cmds = append(*cmds, m.loadDetailSectionsWithFocus(first, ""))
			}
		}
		return true
	case "G":
		if m.focus == 0 {
			m.colList.Select(len(m.colList.Items()) - 1)
			if cmd := m.syncCollectionIndicators(); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
			m.updateActiveMonthFromSelection(false, cmds)
			m.detailRevealTarget = m.selectedCollection()
			*cmds = append(*cmds, m.markCalendarSelection())
			*cmds = append(*cmds, m.loadDetailSectionsWithFocus(m.selectedCollection(), ""))
		} else {
			sections := m.detailState.Sections()
			if len(sections) > 0 {
				last := sections[len(sections)-1].CollectionID
				m.detailState.SetActive(last, "")
				m.selectCollectionByID(last, cmds)
				*cmds = append(*cmds, m.loadDetailSectionsWithFocus(last, ""))
			}
		}
		return true
	case "pgdown":
		if m.focus == 1 {
			if m.moveDetailSection(1, cmds) {
				return true
			}
		}
		return false
	case "pgup":
		if m.focus == 1 {
			if m.moveDetailSection(-1, cmds) {
				return true
			}
		}
		return false
	case "cmd+down":
		if m.focus == 1 {
			if m.moveDetailSection(1, cmds) {
				return true
			}
		}
		return true
	case "cmd+up":
		if m.focus == 1 {
			if m.moveDetailSection(-1, cmds) {
				return true
			}
		}
		return true
	case "[":
		if m.focus == 0 {
			collapse := true
			if cmd := m.toggleFoldCurrent(&collapse); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
			return true
		}
	case "{":
		if m.focus == 0 {
			collapse := true
			if cmd := m.toggleFoldCurrent(&collapse); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
			return true
		}
	case "]":
		if m.focus == 0 {
			expand := false
			if cmd := m.toggleFoldCurrent(&expand); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
			return true
		}
	case "}":
		if m.focus == 0 {
			expand := false
			if cmd := m.toggleFoldCurrent(&expand); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
			return true
		}
	case "shift+tab":
		if m.focus == 1 {
			m.outdentCurrentEntry(cmds)
			return true
		}
	case "tab":
		if m.focus == 1 {
			if it := m.currentEntry(); it != nil {
				defaultParent := m.defaultParentCandidateID()
				m.beginParentSelection(it, defaultParent, cmds)
			}
			return true
		}
	case "o":
		m.pendingChildParent = ""
		m.pendingAddCollection = m.defaultAddCollection()
		m.enterAddMode(cmds)
		return true
	case "O":
		m.pendingChildParent = ""
		if it := m.currentEntry(); it != nil {
			parentID := it.ID
			if it.ParentID != "" {
				parentID = it.ParentID
			}
			m.pendingChildParent = parentID
		}
		m.enterAddMode(cmds)
		return true
	case "i":
		if it := m.currentEntry(); it != nil {
			m.beginEdit(it, cmds)
			return true
		}
	case "x":
		if it := m.currentEntry(); it != nil {
			m.applyComplete(cmds, it)
		}
		return true
	case "d":
		if it := m.currentEntry(); it != nil {
			if m.awaitingDD && time.Since(m.lastDTime) < 600*time.Millisecond {
				m.applyStrikeEntry(cmds, it.ID)
				m.awaitingDD = false
			} else {
				m.awaitingDD = true
				m.lastDTime = time.Now()
			}
		}
		return true
	case ">":
		if it := m.currentEntry(); it != nil {
			if err := m.enterMoveSelector(it, cmds); err != nil {
				m.setStatus("Move: " + err.Error())
			}
			return true
		}
	case "<":
		if it := m.currentEntry(); it != nil {
			m.applyMoveToFuture(cmds, it.ID)
		}
		return true
	case "b":
		var target string
		if m.focus == 1 {
			if it := m.currentEntry(); it != nil {
				target = it.ID
			}
		}
		m.enterBulletSelect(target, menuSectionBullet)
		return true
	case "?":
		m.showHelpPanel()
		return true
	case "v":
		if m.focus == 1 {
			if it := m.currentEntry(); it != nil {
				m.enterBulletSelect(it.ID, menuSectionSignifier)
			} else {
				m.setStatus("Select an entry to edit its signifier")
			}
		} else {
			m.setStatus("Select the entries pane to edit signifiers")
		}
		return true
	case "f1":
		m.showHelpPanel()
		return true
	case "r":
		*cmds = append(*cmds, m.refreshAll())
	case "q":
		m.setStatus("Use :q or :exit to quit")
		return true
	}
	return false
}

func (m *Model) submitInsert(input string, cmds *[]tea.Cmd) {
	switch m.action {
	case actionAdd:
		m.applyAdd(cmds, m.pendingAddCollection, input)
	case actionEdit:
		if it := m.currentEntry(); it != nil {
			m.applyEdit(cmds, it.ID, input)
		}
	}
	m.setMode(modeNormal)
	m.action = actionNone
	m.input.Reset()
	m.input.Blur()
}

func (m *Model) cancelInsert() {
	prevAction := m.action
	m.setMode(modeNormal)
	m.action = actionNone
	m.input.Reset()
	m.input.Blur()
	switch prevAction {
	case actionAdd:
		m.setStatus("Add cancelled")
	case actionEdit:
		m.setStatus("Edit cancelled")
	case actionMove:
		m.setStatus("Move cancelled")
	default:
		m.setStatus("Cancelled")
	}
}

func (m *Model) executeCommand(input string, cmds *[]tea.Cmd) {
	m.commandSelectActive = false
	m.commandOriginalInput = ""
	m.bottom.ClearSuggestion()

	fields := strings.Fields(input)
	if len(fields) == 0 {
		m.setMode(modeNormal)
		m.input.Reset()
		m.input.Blur()
		m.bottom.UpdateCommandInput("", "")
		m.setOverlayReserve(0)
		return
	}
	cmd := strings.ToLower(fields[0])
	args := fields[1:]
	rawArgs := ""
	if len(input) > len(fields[0]) {
		rawArgs = strings.TrimSpace(input[len(fields[0]):])
	}

	switch cmd {
	case "q", "quit", "exit":
		m.stopWatch()
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		*cmds = append(*cmds, tea.Quit)
	case "today":
		if cmd := m.selectToday(); cmd != nil {
			*cmds = append(*cmds, cmd)
		}
	case "future":
		if cmd := m.selectResolvedCollection("Future", "Selected Future"); cmd != nil {
			*cmds = append(*cmds, cmd)
		}
	case "report":
		durationArg := strings.TrimSpace(rawArgs)
		if err := m.launchReportCommand(durationArg, cmds); err != nil {
			m.setStatus("Report: " + err.Error())
		}
		return
	case "migrate":
		durationArg := strings.TrimSpace(rawArgs)
		if err := m.launchMigrationCommand(durationArg, cmds); err != nil {
			m.setStatus("Migration: " + err.Error())
		}
		return
	case "new-collection":
		m.beginCollectionWizard(cmds)
		return
	case "help":
		m.input.Reset()
		m.input.Blur()
		m.bottom.UpdateCommandInput("", "")
		m.showHelpPanel()
		return
	case "type":
		m.handleTypeCommand(rawArgs, cmds)
	case "mkdir":
		m.handleMkdirCommand(rawArgs, cmds)
	case "show-hidden":
		m.handleShowHiddenCommand(args, cmds)
	case "lock":
		m.handleLockCommand(cmds)
	case "unlock":
		m.handleUnlockCommand(cmds)
	case "delete":
		if it := m.currentEntry(); it != nil {
			m.startDeleteConfirm(it, cmds)
		} else {
			m.setStatus("No entry selected to delete")
		}
	case "delete-collection":
		target := strings.TrimSpace(rawArgs)
		if target == "" {
			target = m.activeCollectionCandidate()
		}
		if target == "" {
			m.setStatus("Delete collection: select a collection first")
			break
		}
		m.startCollectionDeleteConfirm(target, cmds)
		return
	case "sweep":
		m.handleSweepCommand(cmds)
		return
	default:
		m.setStatus(fmt.Sprintf("Unknown command: %s", input))
	}
	m.setMode(modeNormal)
	m.input.Reset()
	m.input.Blur()
	m.bottom.UpdateCommandInput("", "")
	m.setOverlayReserve(0)
}

func (m *Model) handleShowHiddenCommand(args []string, cmds *[]tea.Cmd) {
	target := m.showHiddenMoved
	if len(args) == 0 {
		target = !target
	} else {
		switch strings.ToLower(args[0]) {
		case "on", "true", "yes":
			target = true
		case "off", "false", "no":
			target = false
		case "status":
			m.setStatus(fmt.Sprintf("Moved originals currently %s", visibilityLabel(m.showHiddenMoved)))
			return
		default:
			m.setStatus(fmt.Sprintf("Unknown show-hidden option: %s", args[0]))
			return
		}
	}
	if target == m.showHiddenMoved {
		m.setStatus(fmt.Sprintf("Moved originals already %s", visibilityLabel(target)))
		return
	}
	m.showHiddenMoved = target
	if !target && m.panelEntryID != "" {
		if entry := m.findEntryByID(m.panelEntryID); isMovedImmutable(entry) {
			m.closePanel()
		}
	}
	m.setStatus(fmt.Sprintf("Moved originals now %s", visibilityLabel(target)))
	if target {
		*cmds = append(*cmds, m.loadCollections())
	}
	*cmds = append(*cmds, m.loadDetailSections())
}

func (m *Model) handleMkdirCommand(arg string, cmds *[]tea.Cmd) {
	name := strings.TrimSpace(arg)
	if name == "" {
		m.setStatus("mkdir requires a collection name")
		return
	}
	if (strings.HasPrefix(name, "\"") && strings.HasSuffix(name, "\"")) || (strings.HasPrefix(name, "'") && strings.HasSuffix(name, "'")) {
		if len(name) >= 2 {
			name = strings.TrimSpace(name[1 : len(name)-1])
		}
	}
	if name == "" {
		m.setStatus("mkdir requires a collection name")
		return
	}
	target, typ, err := m.ensureCollectionPath(name)
	if err != nil {
		if errors.Is(err, errServiceUnavailable) {
			*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		}
		m.setStatus("ERR: " + err.Error())
		return
	}
	m.setStatus(fmt.Sprintf("Collection created: %s (%s)", target, strings.ToLower(string(typ))))
	m.pendingResolved = target
	*cmds = append(*cmds, m.loadDetailSectionsWithFocus(target, ""))
}

func (m *Model) handleTypeCommand(rawArgs string, cmds *[]tea.Cmd) {
	if m.svc == nil {
		m.setStatus("Type: service unavailable")
		return
	}
	trimmed := strings.TrimSpace(rawArgs)
	if trimmed == "" {
		m.setStatus("Type: provide a collection and type (e.g., :type Future monthly)")
		return
	}
	typeToken := trimmed
	collectionName := ""
	if idx := strings.LastIndex(trimmed, " "); idx >= 0 {
		typeToken = strings.TrimSpace(trimmed[idx+1:])
		collectionName = strings.TrimSpace(trimmed[:idx])
	}
	parsedType, err := collection.ParseType(typeToken)
	if err != nil {
		m.setStatus("Type: " + err.Error())
		return
	}
	if collectionName == "" {
		collectionName = m.selectedCollection()
	}
	if collectionName == "" && m.detailState != nil {
		collectionName = m.detailState.ActiveCollectionID()
	}
	collectionName = strings.TrimSpace(collectionName)
	collectionName = strings.Trim(collectionName, "\"")
	if collectionName == "" {
		m.setStatus("Type: select a collection or specify a name")
		return
	}
	if collectionName == index.TrackingGroupKey {
		m.setStatus("Type: cannot assign type to tracking summary")
		return
	}
	if err := m.svc.EnsureCollectionOfType(m.ctx, collectionName, parsedType); err != nil {
		m.setStatus("Type: " + err.Error())
		return
	}
	m.setStatus(fmt.Sprintf("Set %s to %s", collectionName, strings.ToLower(string(parsedType))))
	m.pendingResolved = collectionName
	cmdList := []tea.Cmd{m.loadCollections(), m.loadDetailSectionsWithFocus(collectionName, "")}
	if cmd := m.syncCollectionIndicators(); cmd != nil {
		cmdList = append(cmdList, cmd)
	}
	*cmds = append(*cmds, tea.Batch(cmdList...))
}

func (m *Model) beginCollectionWizard(cmds *[]tea.Cmd) {
	if m.svc == nil {
		m.setStatus("New collection · service unavailable")
		return
	}
	metas, err := m.svc.CollectionsMeta(m.ctx, "")
	if err != nil {
		m.setStatus("New collection · " + err.Error())
		return
	}
	parentOptions := make([]string, 0, len(metas))
	seen := make(map[string]struct{})
	for _, meta := range metas {
		name := strings.TrimSpace(meta.Name)
		if name == "" {
			continue
		}
		if strings.Contains(name, "/") {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		parentOptions = append(parentOptions, name)
	}
	sort.Strings(parentOptions)
	m.wizard = wizardview.New(m.theme)
	m.wizard.Active = true
	m.wizard.Step = wizardview.StepParent
	m.wizard.ParentOptions = parentOptions
	m.wizard.ParentIndex = 0
	m.wizard.TypeOptions = collection.AllTypes()
	m.wizard.TypeIndex = 0
	m.wizard.Type = collection.TypeGeneric
	m.wizard.SuggestedType = collection.TypeGeneric
	m.bottom.SetCommandPrefix(":new")
	m.bottom.ClearSuggestion()
	m.bottom.SetCommandDefinitions(nil)
	m.input.Reset()
	m.input.Blur()
	m.bottom.UpdateCommandInput("", "")
	m.setMode(modeCollectionWizard)
	m.updateCollectionWizardView()
	m.setStatus("New collection · select parent (↑/↓ move, Enter)")
}

func (m *Model) updateCollectionWizardView() {
	if m.wizard == nil {
		return
	}
	m.bottom.ClearSuggestion()
	switch m.wizard.Step {
	case wizardview.StepParent:
		m.bottom.SetCommandDefinitions(nil)
		m.bottom.SetCommandPrefix("parent> ")
		m.bottom.UpdateCommandInput("", "")
	case wizardview.StepName:
		m.bottom.SetCommandDefinitions(nil)
		m.input.Placeholder = "Collection name"
		m.bottom.SetCommandPrefix("name> ")
		m.bottom.UpdateCommandInput(m.input.Value(), m.input.View())
	case wizardview.StepType:
		if len(m.wizard.TypeOptions) == 0 {
			m.wizard.TypeOptions = collection.AllTypes()
		}
		m.bottom.SetCommandDefinitions(nil)
		if len(m.wizard.TypeOptions) > 0 {
			current := string(m.wizard.TypeOptions[m.wizard.TypeIndex])
			m.bottom.UpdateCommandInput(current, current)
		} else {
			m.bottom.UpdateCommandInput("", "")
		}
		m.bottom.SetCommandPrefix("type> ")
	case wizardview.StepConfirm:
		m.bottom.SetCommandDefinitions(nil)
		m.bottom.UpdateCommandInput("", "")
		m.input.Reset()
		m.input.Blur()
		m.bottom.SetCommandPrefix(":")
	}
}

func (m *Model) advanceCollectionWizard(cmds *[]tea.Cmd) bool {
	if m.wizard == nil {
		return false
	}
	switch m.wizard.Step {
	case wizardview.StepParent:
		options := m.wizard.ParentOptionsWithRoot()
		if m.wizard.ParentIndex < 0 {
			m.wizard.ParentIndex = 0
		}
		if m.wizard.ParentIndex >= len(options) {
			m.wizard.ParentIndex = len(options) - 1
		}
		parentInput := ""
		if m.wizard.ParentIndex > 0 && m.wizard.ParentIndex < len(options) {
			parentInput = options[m.wizard.ParentIndex]
		}
		m.wizard.Parent = parentInput
		m.wizard.Step = wizardview.StepName
		m.input.Reset()
		m.input.Placeholder = "Collection name"
		if parentInput != "" {
			parentType := m.lookupCollectionType(parentInput)
			if parentType == collection.TypeMonthly {
				m.input.SetValue(time.Now().Format("January 2, 2006"))
				m.input.CursorEnd()
			}
		}
		if cmd := m.input.Focus(); cmd != nil {
			*cmds = append(*cmds, cmd)
		}
		*cmds = append(*cmds, textinput.Blink)
		m.updateCollectionWizardView()
		m.setStatus("New collection · enter name")
		return true
	case wizardview.StepName:
		name := strings.TrimSpace(m.input.Value())
		if name == "" {
			m.setStatus("New collection · name is required")
			return true
		}
		parentType := collection.TypeGeneric
		parentLabel := ""
		if m.wizard.Parent != "" {
			parentType = m.lookupCollectionType(m.wizard.Parent)
			parentLabel = uiutil.LastSegment(m.wizard.Parent)
		}
		if err := collection.ValidateChildName(parentType, parentLabel, name); err != nil {
			m.setStatus("New collection · " + err.Error())
			return true
		}
		m.wizard.Name = name
		fullPath := wizardview.JoinPath(m.wizard.Parent, name)
		guess := m.predictCollectionType(fullPath)
		m.wizard.Type = guess
		m.wizard.SuggestedType = guess
		m.wizard.TypeOptions = collection.AllTypes()
		m.wizard.TypeIndex = 0
		for i, typ := range m.wizard.TypeOptions {
			if typ == guess {
				m.wizard.TypeIndex = i
				break
			}
		}
		m.wizard.Step = wizardview.StepType
		m.input.Reset()
		m.input.Blur()
		m.updateCollectionWizardView()
		m.setStatus(fmt.Sprintf("New collection · type %s (Tab to change)", strings.ToLower(string(guess))))
		return true
	case wizardview.StepType:
		if len(m.wizard.TypeOptions) == 0 {
			m.wizard.TypeOptions = collection.AllTypes()
		}
		if m.wizard.TypeIndex < 0 {
			m.wizard.TypeIndex = 0
		}
		if m.wizard.TypeIndex >= len(m.wizard.TypeOptions) {
			m.wizard.TypeIndex = len(m.wizard.TypeOptions) - 1
		}
		m.wizard.Type = m.wizard.TypeOptions[m.wizard.TypeIndex]
		m.wizard.Step = wizardview.StepConfirm
		m.updateCollectionWizardView()
		path := m.wizard.Name
		if m.wizard.Parent != "" && m.wizard.Name != "" {
			path = wizardview.JoinPath(m.wizard.Parent, m.wizard.Name)
		}
		display := m.truncateForStatus(path, 18)
		m.setStatus(fmt.Sprintf("New collection · create %s as %s? (Enter confirm, ctrl+b back, Esc cancel)", display, strings.ToLower(string(m.wizard.Type))))
		return true
	case wizardview.StepConfirm:
		m.finalizeCollectionWizard(cmds)
		return true
	default:
		return false
	}
}

func (m *Model) previousCollectionWizardStep(cmds *[]tea.Cmd) {
	switch m.wizard.Step {
	case wizardview.StepName:
		m.wizard.Step = wizardview.StepParent
		m.input.Reset()
		m.input.Blur()
		m.updateCollectionWizardView()
		m.setStatus("New collection · select parent (↑/↓ move, Enter)")
	case wizardview.StepType:
		m.wizard.Step = wizardview.StepName
		m.input.SetValue(m.wizard.Name)
		m.input.CursorEnd()
		m.updateCollectionWizardView()
		if cmd := m.input.Focus(); cmd != nil {
			if cmds != nil {
				*cmds = append(*cmds, cmd, textinput.Blink)
			}
		}
		m.setStatus("New collection · enter name")
	case wizardview.StepConfirm:
		m.wizard.Step = wizardview.StepType
		m.input.Reset()
		m.input.Blur()
		m.updateCollectionWizardView()
		m.setStatus(fmt.Sprintf("New collection · type %s (Tab to change)", strings.ToLower(string(m.wizard.Type))))
	default:
		m.cancelCollectionWizard("New collection cancelled")
	}
}

func (m *Model) cancelCollectionWizard(message string) {
	m.exitCollectionWizard()
	if message != "" {
		m.setStatus(message)
	}
}

func (m *Model) finalizeCollectionWizard(cmds *[]tea.Cmd) {
	parent := m.wizard.Parent
	name := m.wizard.Name
	if parent == "" && name == "" {
		m.cancelCollectionWizard("New collection cancelled")
		return
	}
	path := wizardview.JoinPath(parent, name)
	if err := m.svc.EnsureCollectionOfType(m.ctx, path, m.wizard.Type); err != nil {
		m.setStatus("New collection · " + err.Error())
		return
	}
	m.pendingResolved = path
	display := m.truncateForStatus(path, 16)
	m.setStatus(fmt.Sprintf("New collection · created %s as %s", display, strings.ToLower(string(m.wizard.Type))))
	m.exitCollectionWizard()
	cmdList := []tea.Cmd{m.loadCollections(), m.loadDetailSectionsWithFocus(path, "")}
	if cmd := m.syncCollectionIndicators(); cmd != nil {
		cmdList = append(cmdList, cmd)
	}
	*cmds = append(*cmds, tea.Batch(cmdList...))
}

func (m *Model) exitCollectionWizard() {
	m.wizard = wizardview.New(m.theme)
	m.bottom.SetCommandPrefix(":")
	m.bottom.ClearSuggestion()
	m.bottom.SetCommandDefinitions(nil)
	m.bottom.UpdateCommandInput("", "")
	m.input.Reset()
	m.input.Blur()
	m.setMode(modeNormal)
	m.setOverlayReserve(0)
}

func truncateMiddle(s string, limit int) string {
	runes := []rune(s)
	if limit <= 0 || len(runes) <= limit {
		return s
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	left := (limit - 1) / 2
	right := limit - left - 1
	return string(runes[:left]) + "…" + string(runes[len(runes)-right:])
}

func (m *Model) truncateForStatus(value string, padding int) string {
	max := m.termWidth
	if max <= 0 {
		max = 80
	}
	limit := max - padding
	if limit < 10 {
		limit = 10
	}
	return truncateMiddle(value, limit)
}

func (m *Model) enterMoveSelector(it *entry.Entry, cmds *[]tea.Cmd) error {
	if it == nil {
		return errors.New("no entry selected")
	}
	if m.svc == nil {
		return errors.New("service unavailable")
	}
	prevMode := m.mode
	collections, err := m.svc.Collections(m.ctx)
	if err != nil {
		return err
	}
	unique := make(map[string]struct{}, len(collections))
	filtered := make([]string, 0, len(collections))
	for _, c := range collections {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, ok := unique[c]; ok {
			continue
		}
		unique[c] = struct{}{}
		filtered = append(filtered, c)
	}
	if len(filtered) == 0 {
		return errors.New("no collections available")
	}
	sort.Strings(filtered)
	m.moveCollections = filtered
	m.moveTargetID = it.ID
	m.commandContext = commandContextMove
	m.commandSelectActive = false
	m.commandOriginalInput = ""
	m.pendingCreateCollection = ""
	m.bottom.ClearSuggestion()
	m.resumeMode = prevMode
	m.setMode(modeCommand)
	m.input.Reset()
	m.input.Placeholder = "Move to collection"
	m.input.Prompt = "> "
	m.input.CursorStart()
	if cmd := m.input.Focus(); cmd != nil {
		*cmds = append(*cmds, cmd)
	}
	*cmds = append(*cmds, textinput.Blink)
	m.bottom.SetCommandPrefix(">")
	m.updateMoveSuggestions("")
	m.setStatus("Move: start typing and tab to autocomplete")
	m.applyReserve()
	return nil
}

func (m *Model) finishMoveSelection(cmds *[]tea.Cmd) error {
	target := strings.TrimSpace(m.input.Value())
	target = strings.TrimSuffix(target, "/")
	if target == "" {
		m.exitMoveSelector(true)
		return errors.New("no collection selected")
	}
	entryID := m.moveTargetID
	if entryID == "" {
		m.exitMoveSelector(true)
		return errors.New("entry unavailable")
	}
	if !m.collectionExists(target) {
		if m.collectionHasChildren(target) {
			withSlash := target
			if !strings.HasSuffix(withSlash, "/") {
				withSlash += "/"
			}
			m.input.SetValue(withSlash)
			m.input.CursorEnd()
			m.bottom.UpdateCommandPreview(m.input.Value(), m.input.View())
			m.updateMoveSuggestions(m.input.Value())
			m.commandSelectActive = false
			m.commandOriginalInput = ""
			m.pendingCreateCollection = ""
			m.pendingCreateType = ""
			m.setStatus("Move: choose a sub-collection")
			return nil
		}
		m.pendingCreateCollection = target
		m.pendingCreateType = m.predictCollectionType(target)
		m.commandSelectActive = false
		m.commandOriginalInput = ""
		typeLabel := strings.ToLower(string(m.pendingCreateType))
		m.setStatus(fmt.Sprintf("Collection %q does not exist. Press Enter to create it as %s, or Esc to cancel.", target, typeLabel))
		return nil
	}
	m.exitMoveSelector(false)
	m.applyMove(cmds, entryID, target)
	return nil
}

func (m *Model) ensureCollectionPath(name string) (string, collection.Type, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", collection.TypeGeneric, errInvalidCollection
	}
	segments := strings.Split(trimmed, "/")
	var (
		paths []string
		parts []string
	)
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		parts = append(parts, segment)
		path := strings.Join(parts, "/")
		if len(paths) == 0 || paths[len(paths)-1] != path {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return "", collection.TypeGeneric, errInvalidCollection
	}
	if m.svc == nil {
		return "", collection.TypeGeneric, errServiceUnavailable
	}
	if err := m.svc.EnsureCollections(m.ctx, paths); err != nil {
		return "", collection.TypeGeneric, err
	}
	resolved := paths[len(paths)-1]
	typ := m.lookupCollectionType(resolved)
	return resolved, typ, nil
}

func (m *Model) lookupCollectionType(name string) collection.Type {
	if m.svc == nil {
		return collection.TypeGeneric
	}
	metas, err := m.svc.CollectionsMeta(m.ctx, name)
	if err != nil {
		return collection.TypeGeneric
	}
	for _, meta := range metas {
		if meta.Name == name {
			if meta.Type == "" {
				return collection.TypeGeneric
			}
			return meta.Type
		}
	}
	return m.predictCollectionType(name)
}

func (m *Model) predictCollectionType(name string) collection.Type {
	if m.svc == nil {
		return collection.TypeGeneric
	}
	all, err := m.svc.CollectionsMeta(m.ctx, "")
	if err != nil {
		return collection.TypeGeneric
	}
	typeMap := make(map[string]collection.Type, len(all))
	for _, meta := range all {
		if meta.Type == "" {
			meta.Type = collection.TypeGeneric
		}
		typeMap[meta.Name] = meta.Type
	}
	trimmed := strings.TrimSpace(name)
	label := trimmed
	parentType := collection.TypeGeneric
	if idx := strings.LastIndex(trimmed, "/"); idx >= 0 {
		parentPath := trimmed[:idx]
		label = strings.TrimSpace(trimmed[idx+1:])
		if typ, ok := typeMap[parentPath]; ok {
			parentType = typ
		}
		parentLabel := strings.TrimSpace(uiutil.LastSegment(parentPath))
		if parentType == collection.TypeGeneric {
			if parentPath == "Future" {
				parentType = collection.TypeMonthly
			} else if collection.IsMonthName(parentLabel) {
				parentType = collection.TypeMonthly
			}
		}
	}
	if trimmed == "Future" {
		return collection.TypeMonthly
	}
	guess := collection.GuessType(label, parentType)
	if parentType == collection.TypeMonthly && guess == collection.TypeGeneric {
		return collection.TypeDaily
	}
	if collection.IsMonthName(label) && guess == collection.TypeGeneric {
		return collection.TypeDaily
	}
	return guess
}

func (m *Model) exitMoveSelector(cancel bool) {
	targetMode := m.resumeMode
	if targetMode == 0 {
		targetMode = modeNormal
	}
	m.commandContext = commandContextGlobal
	m.commandSelectActive = false
	m.commandOriginalInput = ""
	m.bottom.ClearSuggestion()
	m.moveCollections = nil
	m.moveTargetID = ""
	m.pendingCreateCollection = ""
	m.pendingCreateType = ""
	m.input.Reset()
	m.input.Blur()
	m.input.Prompt = ""
	m.bottom.SetCommandPrefix(":")
	m.bottom.UpdateCommandInput("", "")
	m.setOverlayReserve(0)
	m.setMode(targetMode)
	m.resumeMode = modeNormal
	if cancel {
		m.setStatus("Move cancelled")
	}
}

func (m *Model) launchReportCommand(arg string, cmds *[]tea.Cmd) error {
	if m.svc == nil {
		return errors.New("service unavailable")
	}
	duration, label, err := timeutil.ParseWindow(arg)
	if err != nil {
		return err
	}
	until := time.Now()
	since := until.Add(-duration)
	result, err := m.svc.Report(m.ctx, since, until)
	if err != nil {
		return err
	}
	if m.report != nil {
		m.report.SetData(label, since, until, result.Total, result.Sections)
		m.updateReportViewport()
	}
	m.setMode(modeReport)
	m.input.Reset()
	m.input.Blur()
	m.bottom.UpdateCommandInput("", "")
	m.setOverlayReserve(0)
	m.updateBottomContext()
	m.setStatus(fmt.Sprintf("Report · last %s (%d completed)", label, result.Total))
	return nil
}

func (m *Model) launchMigrationCommand(arg string, cmds *[]tea.Cmd) error {
	if m.svc == nil {
		return errors.New("service unavailable")
	}
	duration, label, err := timeutil.ParseWindow(arg)
	if err != nil {
		return err
	}
	until := time.Now()
	since := until.Add(-duration)
	candidates, err := m.svc.MigrationCandidates(m.ctx, since, until)
	if err != nil {
		return err
	}
	m.startMigrationMode(candidates, label, since, until)
	m.input.Reset()
	m.input.Blur()
	m.bottom.UpdateCommandInput("", "")
	return nil
}

func (m *Model) handleLockCommand(cmds *[]tea.Cmd) {
	if m.svc == nil {
		*cmds = append(*cmds, func() tea.Msg { return errMsg{errors.New("service unavailable")} })
		return
	}
	it := m.currentEntry()
	if it == nil {
		m.setStatus("No entry selected to lock")
		return
	}
	collection := it.Collection
	if _, err := m.svc.Lock(m.ctx, it.ID); err != nil {
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		return
	}
	m.setStatus("Locked")
	*cmds = append(*cmds, m.loadDetailSectionsWithFocus(collection, it.ID))
}

func (m *Model) handleUnlockCommand(cmds *[]tea.Cmd) {
	if m.svc == nil {
		*cmds = append(*cmds, func() tea.Msg { return errMsg{errors.New("service unavailable")} })
		return
	}
	it := m.currentEntry()
	if it == nil {
		m.setStatus("No entry selected to unlock")
		return
	}
	collection := it.Collection
	if _, err := m.svc.Unlock(m.ctx, it.ID); err != nil {
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		return
	}
	m.setStatus("Unlocked")
	*cmds = append(*cmds, m.loadDetailSectionsWithFocus(collection, it.ID))
}

// Update handles messages and keybindings
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	skipListRouting := false

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.applySizes()
	case errMsg:
		m.setStatus("ERR: " + msg.err.Error())
	case collectionsLoadedMsg:
		prevResolved := m.currentResolvedCollection()
		m.colList.SetItems(msg.items)
		if len(msg.items) > 0 {
			targetIdx := -1
			if m.pendingResolved != "" {
				targetIdx = indexForResolved(msg.items, m.pendingResolved)
				m.pendingResolved = ""
			}
			if targetIdx == -1 && prevResolved != "" {
				targetIdx = indexForResolved(msg.items, prevResolved)
			}
			if targetIdx == -1 {
				targetIdx = indexForResolved(msg.items, todayResolvedCollection())
			}
			if targetIdx == -1 {
				targetIdx = 0
			}
			m.colList.Select(targetIdx)
			m.updateActiveMonthFromSelection(false, &cmds)
			if _, ok := m.colList.SelectedItem().(*index.CalendarRowItem); ok {
				if cmd := m.markCalendarSelection(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
		if cmd := m.syncCollectionIndicators(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.loadDetailSections())
		m.updateBottomContext()
	case detailSectionsLoadedMsg:
		m.detailState.SetSections(msg.sections)
		visibleSet := msg.visible
		if visibleSet == nil {
			visibleSet = make(map[string]bool, len(msg.sections))
			for _, sec := range msg.sections {
				visibleSet[sec.CollectionID] = len(sec.Entries) > 0
			}
		}
		collection := msg.activeCollection
		entryID := msg.activeEntry
		if collection == "" {
			collection = m.detailState.ActiveCollectionID()
		}
		if collection == "" && len(msg.sections) > 0 {
			collection = msg.sections[0].CollectionID
		}
		m.detailState.SetActive(collection, entryID)
		if active := m.detailState.ActiveCollectionID(); active != "" {
			m.alignCollectionSelection(active, &cmds)
		}
		if m.pendingSweep {
			m.sweepCollections(visibleSet, &cmds, false)
			m.pendingSweep = false
		}
		if target := m.detailRevealTarget; target != "" {
			preferFull := m.focus == 0
			m.detailState.RevealCollection(target, preferFull, m.detailHeight)
			m.detailRevealTarget = ""
		}
		if m.mode == modePanel && m.panelEntryID != "" {
			if entry := m.findEntryByID(m.panelEntryID); entry != nil {
				m.populateTaskPanel(entry)
			} else {
				m.closePanel()
			}
		}
		m.updateBottomContext()
	case detailFocusChangedMsg:
		skipListRouting = true
		if msg.Collection == "" {
			break
		}
		m.pendingResolved = msg.Collection
		if msg.FromDetail {
			m.detailRevealTarget = msg.Collection
			m.updateCalendarSelection(msg.Collection, &cmds)
			m.alignCollectionSelection(msg.Collection, &cmds)
		}
		m.updateBottomContext()
	case watchStartedMsg:
		if msg.err != nil {
			m.setStatus("ERR: watch " + msg.err.Error())
			break
		}
		m.stopWatch()
		m.watchCh = msg.ch
		m.watchCancel = msg.cancel
		m.setStatus("Watching for changes")
		if cmd := m.waitForWatch(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case watchEventMsg:
		m.handleWatchEvent(msg.event, &cmds)
		if cmd := m.waitForWatch(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case watchStoppedMsg:
		m.stopWatch()
		cmds = append(cmds, startWatchCmd(m.ctx, m.svc))
	case tea.KeyPressMsg:
		skipListRouting = m.handleKeyPress(msg, &cmds)
	}

	// route lists updates depending on focus
	if m.mode == modeNormal && !skipListRouting {
		if m.focus == 0 {
			prev := m.selectedCollection()
			var cmd tea.Cmd
			m.colList, cmd = m.colList.Update(msg)
			cmds = append(cmds, cmd)
			m.updateActiveMonthFromSelection(false, &cmds)
			if newSel := m.selectedCollection(); newSel != prev {
				cmds = append(cmds, m.loadDetailSections())
				m.updateBottomContext()
			}
			if cmd := m.syncCollectionIndicators(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) currentEntry() *entry.Entry {
	if m.detailState == nil {
		return nil
	}
	sections := m.detailState.Sections()
	if len(sections) == 0 {
		return nil
	}
	sectionIdx, entryIdx := m.detailState.Cursor()
	if sectionIdx < 0 || sectionIdx >= len(sections) {
		return nil
	}
	section := sections[sectionIdx]
	if entryIdx < 0 || entryIdx >= len(section.Entries) {
		return nil
	}
	return section.Entries[entryIdx]
}

func (m *Model) applyAdd(cmds *[]tea.Cmd, collection, message string) {
	if strings.TrimSpace(message) == "" {
		return
	}
	target := collection
	if target == "" {
		target = m.pendingAddCollection
	}
	if target == "" {
		if m.detailState != nil {
			if active := m.detailState.ActiveCollectionID(); active != "" {
				target = active
			}
		}
	}
	if target == "" {
		target = m.selectedCollection()
	}
	if target == "" {
		return
	}
	if m.svc == nil {
		*cmds = append(*cmds, func() tea.Msg { return errMsg{errors.New("service unavailable")} })
		return
	}
	entry, err := m.svc.Add(m.ctx, target, m.pendingBullet, message, glyph.None)
	if err != nil {
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		return
	}
	parentID := m.pendingChildParent
	m.pendingChildParent = ""
	m.pendingAddCollection = ""
	if parentID != "" && entry != nil {
		if _, err := m.svc.SetParent(m.ctx, entry.ID, parentID); err != nil {
			*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		}
	}
	m.invalidateCollectionCache(target)
	entryID := ""
	if entry != nil {
		entryID = entry.ID
	}
	*cmds = append(*cmds, m.loadDetailSectionsWithFocus(target, entryID))
	m.setStatus("Added")
}

func (m *Model) applyEdit(cmds *[]tea.Cmd, id, message string) {
	if id == "" || message == "" {
		return
	}
	if _, err := m.svc.Edit(m.ctx, id, message); err != nil {
		if m.handleImmutableError(err) {
			return
		}
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		return
	}
	m.setStatus("Edited")
	*cmds = append(*cmds, m.refreshAll())
}

func (m *Model) applyMove(cmds *[]tea.Cmd, id, target string) {
	if id == "" || strings.TrimSpace(target) == "" {
		return
	}
	clone, err := m.svc.Move(m.ctx, id, target)
	if err != nil {
		if m.handleImmutableError(err) {
			return
		}
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		return
	}
	if m.migration != nil && m.migration.Active {
		m.handleMigrationAfterAction(id, clone)
	}
	name := uiutil.FormattedCollectionName(target)
	if clone != nil && strings.TrimSpace(clone.Collection) != "" {
		name = uiutil.FormattedCollectionName(clone.Collection)
	}
	if strings.TrimSpace(name) == "" {
		name = target
	}
	m.setStatus("Moved to " + name)
	*cmds = append(*cmds, m.refreshAll())
}

func (m *Model) applyMoveToFuture(cmds *[]tea.Cmd, id string) {
	if id == "" {
		return
	}
	clone, err := m.svc.Move(m.ctx, id, "Future")
	if err != nil {
		if m.handleImmutableError(err) {
			return
		}
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		return
	}
	if m.migration != nil && m.migration.Active {
		m.handleMigrationAfterAction(id, clone)
	}
	m.setStatus("Moved to Future")
	*cmds = append(*cmds, m.refreshAll())
}

func (m *Model) applyComplete(cmds *[]tea.Cmd, ent *entry.Entry) {
	if ent == nil || ent.ID == "" {
		return
	}
	if ent.Bullet != glyph.Task {
		m.setStatus("Only tasks can be completed")
		return
	}
	updated, err := m.svc.Complete(m.ctx, ent.ID)
	if err != nil {
		if m.handleImmutableError(err) {
			return
		}
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		return
	}
	if m.migration != nil && m.migration.Active {
		m.handleMigrationAfterAction(ent.ID, updated)
	}
	m.setStatus("Completed")
	*cmds = append(*cmds, m.refreshAll())
}

func (m *Model) applyStrikeEntry(cmds *[]tea.Cmd, id string) {
	if id == "" {
		return
	}
	updated, err := m.svc.Strike(m.ctx, id)
	if err != nil {
		if m.handleImmutableError(err) {
			return
		}
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		return
	}
	if m.migration != nil && m.migration.Active {
		m.handleMigrationAfterAction(id, updated)
	}
	m.setStatus("Struck")
	*cmds = append(*cmds, m.refreshAll())
}

func (m *Model) applySetBullet(cmds *[]tea.Cmd, id string, b glyph.Bullet) {
	if _, err := m.svc.SetBullet(m.ctx, id, b); err != nil {
		if m.handleImmutableError(err) {
			return
		}
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		return
	}
	m.setStatus("Bullet updated")
	*cmds = append(*cmds, m.refreshAll())
}

func (m *Model) applySetSignifier(cmds *[]tea.Cmd, id string, sig glyph.Signifier) {
	if id == "" {
		return
	}
	entry, err := m.svc.SetSignifier(m.ctx, id, sig)
	if err != nil {
		if m.handleImmutableError(err) {
			return
		}
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		return
	}
	label := sig.Glyph().Meaning
	if sig == glyph.None || strings.TrimSpace(label) == "" {
		label = "none"
	}
	m.setStatus(fmt.Sprintf("Signifier set to %s", label))
	collection := ""
	if entry != nil {
		collection = entry.Collection
	}
	if collection == "" {
		collection = m.detailState.ActiveCollectionID()
		if collection == "" {
			collection = m.selectedCollection()
		}
	}
	if m.mode == modePanel {
		m.panelEntryID = id
		m.panelCollection = collection
	}
	*cmds = append(*cmds, m.loadDetailSectionsWithFocus(collection, id))
}

func (m *Model) enterAddMode(cmds *[]tea.Cmd) {
	m.setMode(modeInsert)
	m.action = actionAdd
	m.input.Placeholder = "New item message"
	m.input.SetValue("")
	if cmd := m.input.Focus(); cmd != nil {
		*cmds = append(*cmds, cmd)
	}
	*cmds = append(*cmds, textinput.Blink)
}

func (m *Model) applySetParent(cmds *[]tea.Cmd, childID, parentID string) {
	if childID == "" || m.svc == nil {
		return
	}
	entry, err := m.svc.SetParent(m.ctx, childID, parentID)
	if err != nil {
		if m.handleImmutableError(err) {
			return
		}
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		m.setStatus("ERR: " + err.Error())
		return
	}
	collection := ""
	if entry != nil {
		collection = entry.Collection
	}
	if collection == "" {
		collection = m.detailState.ActiveCollectionID()
		if collection == "" {
			collection = m.selectedCollection()
		}
	}
	m.invalidateCollectionCache(collection)
	if parentID == "" {
		m.setStatus("Outdented")
	} else {
		label := ""
		if parent := m.findEntryByID(parentID); parent != nil {
			label = uiutil.EntryLabel(parent)
		}
		if label == "" {
			label = "parent"
		}
		m.setStatus("Indented under " + label)
	}
	*cmds = append(*cmds, m.loadDetailSectionsWithFocus(collection, childID))
}

func (m *Model) outdentCurrentEntry(cmds *[]tea.Cmd) {
	if m.detailState == nil {
		return
	}
	it := m.currentEntry()
	if it == nil || it.ParentID == "" {
		m.setStatus("No parent to remove")
		return
	}
	newParent := ""
	if parent := m.findEntryByID(it.ParentID); parent != nil {
		newParent = parent.ParentID
	}
	m.applySetParent(cmds, it.ID, newParent)
}

func (m *Model) defaultParentCandidateID() string {
	if m.detailState == nil {
		return ""
	}
	sections := m.detailState.Sections()
	secIdx, entryIdx := m.detailState.Cursor()
	if secIdx < 0 || secIdx >= len(sections) {
		return ""
	}
	section := sections[secIdx]
	for i := entryIdx - 1; i >= 0; i-- {
		prev := section.Entries[i]
		if prev == nil {
			continue
		}
		if prev.ParentID == "" {
			return prev.ID
		}
		return prev.ParentID
	}
	return ""
}

func (m *Model) defaultAddCollection() string {
	if m.detailState != nil {
		if active := m.detailState.ActiveCollectionID(); active != "" {
			return active
		}
	}
	if sel := m.selectedCollection(); sel != "" {
		return sel
	}
	return ""
}

func (m *Model) beginParentSelection(entry *entry.Entry, defaultParent string, cmds *[]tea.Cmd) {
	if entry == nil {
		return
	}
	candidates, err := m.buildParentCandidates(entry.Collection, entry.ID)
	if err != nil {
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		m.setStatus("ERR: " + err.Error())
		return
	}
	if len(candidates) == 0 {
		m.setStatus("No available parents")
		return
	}
	idx := 0
	if defaultParent != "" {
		for i, cand := range candidates {
			if cand.ID == defaultParent {
				idx = i
				break
			}
		}
	}
	m.parentSelect = parentSelectState{
		active:     true,
		childID:    entry.ID,
		collection: entry.Collection,
		candidates: candidates,
		index:      idx,
	}
	m.setMode(modeParentSelect)
	m.updateParentSelectStatus()
}

func (m *Model) buildParentCandidates(collection string, childID string) ([]parentCandidate, error) {
	entries, err := m.entriesForCollection(collection)
	if err != nil {
		return nil, err
	}
	entries, _ = m.filterEntriesForDisplay(entries)
	blocked := descendantIDs(entries, childID)
	blocked[childID] = struct{}{}
	candidates := []parentCandidate{{ID: "", Label: "<root>"}}
	for _, e := range entries {
		if e == nil || e.ID == "" {
			continue
		}
		if _, ok := blocked[e.ID]; ok {
			continue
		}
		if e.ParentID != "" {
			continue
		}
		candidates = append(candidates, parentCandidate{ID: e.ID, Label: uiutil.EntryLabel(e)})
	}
	return candidates, nil
}

func descendantIDs(entries []*entry.Entry, rootID string) map[string]struct{} {
	children := make(map[string][]string)
	for _, e := range entries {
		if e == nil || e.ID == "" {
			continue
		}
		if e.ParentID == "" {
			continue
		}
		children[e.ParentID] = append(children[e.ParentID], e.ID)
	}
	visited := make(map[string]struct{})
	var walk func(string)
	walk = func(id string) {
		for _, child := range children[id] {
			if _, ok := visited[child]; ok {
				continue
			}
			visited[child] = struct{}{}
			walk(child)
		}
	}
	walk(rootID)
	return visited
}

func (m *Model) handleParentSelectKey(msg tea.KeyPressMsg, cmds *[]tea.Cmd) bool {
	if !m.parentSelect.active {
		m.exitParentSelect()
		return true
	}
	if len(m.parentSelect.candidates) == 0 {
		m.exitParentSelect()
		return true
	}
	switch msg.String() {
	case "esc":
		m.exitParentSelect()
		return true
	case "enter":
		m.confirmParentSelection(cmds)
		return true
	case "up", "k":
		m.parentSelect.index--
		if m.parentSelect.index < 0 {
			m.parentSelect.index = len(m.parentSelect.candidates) - 1
		}
		m.updateParentSelectStatus()
		return true
	case "down", "j":
		m.parentSelect.index++
		if m.parentSelect.index >= len(m.parentSelect.candidates) {
			m.parentSelect.index = 0
		}
		m.updateParentSelectStatus()
		return true
	default:
		return false
	}
}

func (m *Model) updateParentSelectStatus() {
	if !m.parentSelect.active || len(m.parentSelect.candidates) == 0 {
		return
	}
	idx := m.parentSelect.index
	if idx < 0 {
		idx = 0
	}
	if idx >= len(m.parentSelect.candidates) {
		idx = len(m.parentSelect.candidates) - 1
	}
	label := m.parentSelect.candidates[idx].Label
	if label == "" {
		label = "<root>"
	}
	m.setStatus("Parent → " + label)
}

func (m *Model) exitParentSelect() {
	m.parentSelect = parentSelectState{}
	m.setMode(modeNormal)
	m.updateBottomContext()
}

func (m *Model) confirmParentSelection(cmds *[]tea.Cmd) {
	if !m.parentSelect.active || len(m.parentSelect.candidates) == 0 {
		m.exitParentSelect()
		return
	}
	cand := m.parentSelect.candidates[m.parentSelect.index]
	childID := m.parentSelect.childID
	m.exitParentSelect()
	m.applySetParent(cmds, childID, cand.ID)
}

// View renders two lists and optional input/help overlays
func (m *Model) View() string {
	var sections []string

	switch m.mode {
	case modeCollectionWizard:
		if m.wizard != nil && m.wizard.Active {
			m.wizard.SetSize(m.termWidth, m.termHeight)
			m.wizard.SetNameInputView(m.input.View())
			if overlay := m.wizard.View(); strings.TrimSpace(overlay) != "" {
				sections = append(sections, overlay)
				m.setOverlayReserve(m.wizard.OverlayReserve())
			}
		}
	case modeReport:
		if m.report != nil {
			if overlay := m.report.View(); strings.TrimSpace(overlay) != "" {
				sections = append(sections, overlay)
			}
		}
	case modeMigration:
		if m.migration != nil && m.migration.Active {
			m.updateMigrationViewport()
			if view := m.migration.View(); strings.TrimSpace(view) != "" {
				sections = append(sections, view)
			}
		}
	default:
		left := m.colList.View()
		right := m.renderDetailPane()
		gap := lipgloss.NewStyle().Padding(0, 1).Render
		body := lipgloss.JoinHorizontal(lipgloss.Top, left, gap(" "), right)
		sections = append(sections, body)
	}

	if m.mode == modeInsert {
		prompt := ""
		switch m.action {
		case actionAdd:
			prompt = "Add: "
		case actionEdit:
			prompt = "Edit: "
		case actionMove:
			prompt = "Move: "
		}
		sections = append(sections, prompt+m.input.View())
	}
	if m.mode == modeBulletSelect {
		panelStyle := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(1, 2)
		sections = append(sections, panelStyle.Render(strings.Join(m.bulletMenuLines(), "\n")))
	}
	if m.mode == modeHelp && m.mode != modeReport {
		helpLines := m.helpLines
		if len(helpLines) == 0 {
			helpLines = buildHelpLines()
		}
		panelStyle := lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).Padding(1, 2)
		sections = append(sections, panelStyle.Render(strings.Join(helpLines, "\n")))
	}
	if m.mode == modeConfirm {
		sections = append(sections, "Confirm delete (type yes): "+m.input.View())
	}
	if m.mode == modePanel && m.panelEntryID != "" {
		if view, _ := m.panelModel.View(); strings.TrimSpace(view) != "" {
			sections = append(sections, view)
		}
	}
	if footer, _ := m.bottom.View(); footer != "" {
		sections = append(sections, footer)
	}

	return strings.Join(sections, "\n\n")
}

func (m *Model) renderDetailPane() string {
	contentHeight := m.detailHeight
	if contentHeight <= 0 {
		contentHeight = 1
	}
	if m.detailState == nil {
		return placeholderDetail(m.focus == 1)
	}
	view, _ := m.detailState.Viewport(contentHeight)
	if strings.TrimSpace(view) == "" {
		return placeholderDetail(m.focus == 1)
	}
	return view
}

// Run launches the interactive TUI program.
func Run(svc *app.Service) error {
	p := tea.NewProgram(New(svc), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// applySizes recalculates list sizes based on current terminal size.
func (m *Model) applySizes() {
	if m.termWidth == 0 || m.termHeight == 0 {
		return
	}
	// Allocate ~1/3 for collections with sensible bounds.
	left := m.termWidth / 3
	if left < 24 {
		left = 24
	}
	if left > 40 {
		left = 40
	}
	// Space for gap and borders
	right := m.termWidth - left - 4
	if right < 20 {
		right = 20
	}
	// Leave room for status/footer lines
	height := m.termHeight - 4 - m.verticalReserve
	if height < 5 {
		height = 5
	}
	m.colList.SetSize(left, height)
	m.detailWidth = right
	m.detailHeight = height
	if m.detailState != nil {
		m.detailState.SetWrapWidth(right)
	}
	m.updateReportViewport()
	m.updateMigrationViewport()
}

func (m *Model) updateReportViewport() {
	if m.report == nil {
		return
	}
	if m.termWidth == 0 || m.termHeight == 0 {
		return
	}
	height := m.termHeight - m.bottom.Height() - 4
	m.report.SetViewport(m.termWidth, height)
}

func (m *Model) updateMigrationViewport() {
	if m.migration == nil {
		return
	}
	if m.termWidth == 0 {
		return
	}
	height := m.detailHeight
	if height <= 0 {
		height = m.termHeight - m.bottom.Height() - 4
		if height < 5 {
			height = 5
		}
	}
	m.migration.SetSize(m.termWidth, height)
}

func (m *Model) applyReserve() {
	total := m.overlayReserve + m.bottom.ExtraHeight()
	if total == m.verticalReserve {
		return
	}
	m.verticalReserve = total
	m.applySizes()
}

func (m *Model) setOverlayReserve(lines int) {
	if lines < 0 {
		lines = 0
	}
	if m.overlayReserve == lines {
		return
	}
	m.overlayReserve = lines
	m.applyReserve()
}

func (m *Model) mapBottomMode(md mode) bottombar.Mode {
	switch md {
	case modeInsert:
		return bottombar.ModeInsert
	case modeCommand:
		return bottombar.ModeCommand
	case modeCollectionWizard:
		return bottombar.ModeCommand
	case modeHelp:
		return bottombar.ModeHelp
	case modeBulletSelect:
		return bottombar.ModeBulletSelect
	case modePanel:
		return bottombar.ModeHelp
	case modeConfirm:
		return bottombar.ModeInsert
	case modeParentSelect:
		return bottombar.ModeCommand
	case modeReport:
		return bottombar.ModeHelp
	case modeMigration:
		return bottombar.ModeHelp
	default:
		return bottombar.ModeNormal
	}
}

func (m *Model) setMode(newMode mode) {
	m.mode = newMode
	m.bottom.SetMode(m.mapBottomMode(newMode))
	m.updateBottomContext()
	m.applyReserve()
}

func (m *Model) setStatus(msg string) {
	m.bottom.SetStatus(msg)
}

func (m *Model) updateBottomContext() {
	var help string
	switch m.mode {
	case modeCommand:
		if m.commandContext == commandContextMove {
			help = "Move · Tab cycle · Enter confirm · Esc cancel"
		} else {
			help = ""
		}
	case modeCollectionWizard:
		if m.wizard != nil {
			switch m.wizard.Step {
			case wizardview.StepParent:
				help = "New collection · Tab cycle parents · Enter continue · Esc cancel"
			case wizardview.StepName:
				help = "New collection · Enter continue · ctrl+b back · Esc cancel"
			case wizardview.StepType:
				help = "New collection · j/k or Tab cycle types · Enter continue · ctrl+b back · Esc cancel"
			case wizardview.StepConfirm:
				help = "New collection · Enter create · ctrl+b back · Esc cancel"
			default:
				help = "New collection"
			}
		}
	case modeInsert:
		switch m.action {
		case actionAdd:
			help = "Compose · Enter save · Esc cancel · ctrl+b bullet/signifier menu"
		case actionEdit:
			help = "Edit · Enter save · Esc cancel"
		case actionMove:
			help = "Move · Enter confirm · Esc cancel"
		default:
			help = "Compose · Enter save · Esc cancel"
		}
	case modeHelp:
		help = "Help · esc close"
	case modeBulletSelect:
		help = "Select bullet/signifier · Enter confirm · Esc cancel · j/k move · Tab switch section"
	case modePanel:
		help = "Task detail · enter/esc close · e edit · b bullet/signifier menu · v signifier menu"
	case modeConfirm:
		help = "Confirm delete · type yes · enter confirm · esc cancel"
	case modeParentSelect:
		help = "Select parent · ↑/↓ choose · Enter confirm · Esc cancel"
	case modeReport:
		help = "Report · j/k scroll · PgUp/PgDn page · g/G home/end · space page · q/esc close"
	default:
		if m.focus == 0 {
			if m.isCalendarActive() {
				help = "Index · h/l day · j/k week · enter focus · o add entry · { fold · } expand month · F1 help"
			} else {
				help = "Index · h/l panes · j/k move · o add entry · { collapse · } expand · : command mode · F1 help"
			}
		} else {
			hiddenState := "off"
			if m.showHiddenMoved {
				hiddenState = "on"
			}
			help = fmt.Sprintf("Entries · j/k move · PgUp/PgDn or cmd+↑/↓ switch collection · o add · O add child · tab indent · shift+tab outdent · i edit · x complete · dd strike · b bullet/signifier menu · v signifier menu · > move · :mkdir make collection · :lock lock · :unlock unlock · :help guide · :show-hidden toggle (now %s)", hiddenState)
		}
	case modeMigration:
		switch {
		case m.migration == nil || len(m.migration.Items) == 0:
			help = "Migration · No open tasks · esc exit"
		case m.migration.Focus == migrationview.FocusTasks:
			help = "Migration · j/k move · > choose target · < future · x complete · delete strike · esc exit"
		default:
			help = "Targets · j/k choose · enter migrate · > migrate · esc back"
		}
	}
	m.bottom.SetHelp(help)
}

func (m *Model) isCalendarActive() bool {
	sel := m.colList.SelectedItem()
	if sel == nil {
		return false
	}
	switch sel.(type) {
	case *index.CalendarRowItem, *index.CalendarHeaderItem:
		return true
	default:
		return false
	}
}

// updateFocusHeaders updates pane titles to reflect which pane is focused.
func (m *Model) updateFocusHeaders() {
	if m.focus == 0 {
		m.colList.SetDelegate(m.focusDel)
	} else {
		m.colList.SetDelegate(m.blurDel)
	}
}

func (m *Model) findBulletIndex(b glyph.Bullet) int {
	for i, opt := range m.bulletOptions {
		if opt == b {
			return i
		}
	}
	return 0
}

func (m *Model) buildBulletMenuOptions(includeSignifiers bool) []bulletMenuOption {
	options := make([]bulletMenuOption, 0, len(m.bulletOptions)+len(m.signifierOptions))
	for _, b := range m.bulletOptions {
		options = append(options, bulletMenuOption{
			section: menuSectionBullet,
			bullet:  b,
		})
	}
	if includeSignifiers {
		for _, s := range m.signifierOptions {
			options = append(options, bulletMenuOption{
				section:   menuSectionSignifier,
				signifier: s,
			})
		}
	}
	return options
}

func (m *Model) menuFirstIndex(section menuSection) int {
	for i, opt := range m.bulletMenuOptions {
		if opt.section == section {
			return i
		}
	}
	return -1
}

func (m *Model) findMenuIndexForBullet(b glyph.Bullet) int {
	for i, opt := range m.bulletMenuOptions {
		if opt.section == menuSectionBullet && opt.bullet == b {
			return i
		}
	}
	return -1
}

func (m *Model) findMenuIndexForSignifier(s glyph.Signifier) int {
	for i, opt := range m.bulletMenuOptions {
		if opt.section == menuSectionSignifier && opt.signifier == s {
			return i
		}
	}
	return -1
}

func (m *Model) enterBulletSelect(targetID string, focus menuSection) {
	prevMode := m.mode
	m.setMode(modeBulletSelect)
	m.resumeMode = prevMode
	if prevMode == modeInsert {
		m.input.Blur()
	}
	m.bulletTargetID = targetID
	m.bulletMenuFocus = focus

	currentBullet := m.pendingBullet
	currentSignifier := glyph.None
	includeSignifiers := false
	if targetID != "" {
		if entry := m.findEntryByID(targetID); entry != nil {
			currentBullet = entry.Bullet
			currentSignifier = entry.Signifier
		}
		includeSignifiers = true
	}

	m.bulletMenuOptions = m.buildBulletMenuOptions(includeSignifiers)
	if len(m.bulletMenuOptions) == 0 {
		m.setStatus("No options available")
		return
	}

	var idx int
	switch focus {
	case menuSectionSignifier:
		if includeSignifiers {
			idx = m.findMenuIndexForSignifier(currentSignifier)
			if idx < 0 {
				idx = m.menuFirstIndex(menuSectionSignifier)
			}
		} else {
			idx = m.menuFirstIndex(menuSectionBullet)
		}
	default:
		idx = m.findMenuIndexForBullet(currentBullet)
		if idx < 0 {
			idx = m.menuFirstIndex(menuSectionBullet)
		}
	}
	if idx < 0 {
		idx = 0
	}
	m.bulletIndex = idx
	m.bulletMenuFocus = m.bulletMenuOptions[m.bulletIndex].section

	reserve := len(m.bulletMenuOptions) + 8
	m.setOverlayReserve(reserve)
	if includeSignifiers {
		m.setStatus("Select bullet or signifier")
	} else {
		m.setStatus("Select default bullet for new entries")
	}
}

func (m *Model) bulletMenuLines() []string {
	lines := []string{"Select bullet or signifier · Enter confirm · Esc cancel · Tab switch section"}
	if len(m.bulletMenuOptions) == 0 {
		lines = append(lines, "", "(no options available)")
		return lines
	}
	currentSection := menuSection(-1)
	for i, opt := range m.bulletMenuOptions {
		if opt.section != currentSection {
			currentSection = opt.section
			lines = append(lines, "")
			switch currentSection {
			case menuSectionBullet:
				lines = append(lines, "Bullets:")
			case menuSectionSignifier:
				lines = append(lines, "Signifiers:")
			}
		}
		indicator := "  "
		if i == m.bulletIndex {
			indicator = "→ "
		}
		var label string
		switch opt.section {
		case menuSectionBullet:
			glyphInfo := opt.bullet.Glyph()
			symbol := strings.TrimSpace(glyphInfo.Symbol)
			if symbol == "" {
				symbol = opt.bullet.String()
			}
			label = fmt.Sprintf("%s %s", symbol, glyphInfo.Meaning)
		case menuSectionSignifier:
			glyphInfo := opt.signifier.Glyph()
			symbol := strings.TrimSpace(glyphInfo.Symbol)
			meaning := glyphInfo.Meaning
			if opt.signifier == glyph.None {
				symbol = "·"
				meaning = "None"
			}
			if symbol == "" {
				symbol = opt.signifier.String()
			}
			label = fmt.Sprintf("%s %s", symbol, meaning)
		}
		lines = append(lines, fmt.Sprintf("%s%s", indicator, label))
	}
	return lines
}

func (m *Model) exitBulletSelect(cmds *[]tea.Cmd) {
	target := m.resumeMode
	if target == 0 {
		target = modeNormal
	}
	m.setMode(target)
	if target == modeInsert {
		if cmd := m.input.Focus(); cmd != nil {
			*cmds = append(*cmds, cmd)
		}
		*cmds = append(*cmds, textinput.Blink)
	}
	m.bulletTargetID = ""
	m.resumeMode = modeNormal
	m.bulletMenuOptions = nil
	m.bulletMenuFocus = menuSectionBullet
	m.setOverlayReserve(0)
}

func (m *Model) enterCommandMode(cmds *[]tea.Cmd) {
	m.setMode(modeCommand)
	m.commandContext = commandContextGlobal
	m.commandSelectActive = false
	m.commandOriginalInput = ""
	m.bottom.ClearSuggestion()
	m.input.Reset()
	m.input.Placeholder = "command"
	m.input.CursorEnd()
	if cmd := m.input.Focus(); cmd != nil {
		*cmds = append(*cmds, cmd)
	}
	*cmds = append(*cmds, textinput.Blink)
	m.bottom.SetCommandPrefix(":")
	m.bottom.SetCommandDefinitions(commandDefinitions)
	m.bottom.UpdateCommandInput(m.input.Value(), m.input.View())
	m.setStatus("COMMAND: :q quit · :today Today · :future Future · :report window · :type set type")
	m.applyReserve()
}

func (m *Model) selectToday() tea.Cmd {
	monthLabel, dayLabel, resolved := todayLabels()
	if t, err := time.Parse("January 2, 2006", dayLabel); err == nil {
		if m.indexState.Selection == nil {
			m.indexState.Selection = make(map[string]int)
		}
		m.indexState.Selection[monthLabel] = t.Day()
	}
	m.indexState.Fold[monthLabel] = false
	return m.selectResolvedCollection(resolved, fmt.Sprintf("Selected Today (%s)", dayLabel))
}

func (m *Model) selectResolvedCollection(resolved, status string) tea.Cmd {
	if resolved == "" {
		return nil
	}
	m.pendingResolved = resolved
	m.focus = 1
	m.updateFocusHeaders()
	m.updateBottomContext()
	m.setOverlayReserve(0)
	m.detailRevealTarget = resolved
	if status == "" {
		m.setStatus(fmt.Sprintf("Selected %s", resolved))
	} else {
		m.setStatus(status)
	}
	cmds := []tea.Cmd{m.loadCollections(), m.loadDetailSectionsWithFocus(resolved, "")}
	if cmd := m.syncCollectionIndicators(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func (m *Model) toggleFoldCurrent(explicit *bool) tea.Cmd {
	if m.focus == 1 {
		return nil
	}
	if m.focus != 0 {
		return nil
	}
	sel := m.colList.SelectedItem()
	if sel == nil {
		return nil
	}
	key := ""
	switch v := sel.(type) {
	case index.CollectionItem:
		if v.Indent {
			if v.Resolved == "" {
				return nil
			}
			parts := strings.SplitN(v.Resolved, "/", 2)
			if len(parts) == 0 {
				return nil
			}
			key = parts[0]
		} else {
			if !v.HasChildren {
				return nil
			}
			key = v.Resolved
			if key == "" {
				key = v.Name
			}
		}
	case *index.CalendarRowItem:
		key = v.Month
	case *index.CalendarHeaderItem:
		key = v.Month
	default:
		return nil
	}
	if key == "" {
		return nil
	}
	current := m.indexState.Fold[key]
	var desired bool
	if explicit == nil {
		desired = !current
	} else {
		desired = *explicit
		if current == desired {
			return nil
		}
	}
	m.indexState.Fold[key] = desired
	m.pendingResolved = key
	return m.loadCollections()
}

func (m *Model) syncCollectionIndicators() tea.Cmd {
	items := m.colList.Items()
	if len(items) == 0 {
		return nil
	}
	activeIdx := m.colList.Index()
	if activeIdx < 0 || activeIdx >= len(items) {
		activeIdx = -1
	}
	var cmds []tea.Cmd
	for i, it := range items {
		ci, ok := it.(index.CollectionItem)
		if !ok {
			continue
		}
		wantActive := i == activeIdx
		if ci.Active == wantActive {
			continue
		}
		ci.Active = wantActive
		if cmd := m.colList.SetItem(i, ci); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) currentResolvedCollection() string {
	sel := m.colList.SelectedItem()
	if sel == nil {
		return ""
	}
	ci, ok := sel.(index.CollectionItem)
	if !ok {
		return ""
	}
	if ci.Resolved != "" {
		return ci.Resolved
	}
	return ci.Name
}

func indexForResolved(items []list.Item, resolved string) int {
	for i, it := range items {
		switch v := it.(type) {
		case index.CollectionItem:
			if v.Resolved == resolved || (v.Resolved == "" && v.Name == resolved) {
				return i
			}
		case *index.CalendarHeaderItem:
			if resolved == v.Month {
				return i
			}
		case *index.CalendarRowItem:
			if strings.HasPrefix(resolved, v.Month+"/") {
				if day := index.DayFromPath(resolved); day > 0 && index.ContainsDay(v.Days, day) {
					return i
				}
			}
		}
	}
	return -1
}

func indexForName(items []list.Item, name string) int {
	for i, it := range items {
		ci, ok := it.(index.CollectionItem)
		if !ok {
			continue
		}
		if ci.Name == name {
			return i
		}
	}
	return -1
}

func (m *Model) buildCollectionItems(metas []collection.Meta, currentResolved string, now time.Time) []list.Item {
	return index.BuildItems(m.indexState, metas, currentResolved, now)
}

func (m *Model) buildDetailOrder() []collectionDescriptor {
	type orderedDesc struct {
		desc  collectionDescriptor
		order int
	}

	const parentStep = 10000
	const fallbackStart = 1000

	orders := make([]orderedDesc, 0, len(m.colList.Items()))
	seen := make(map[string]bool)
	parentOrder := make(map[string]int)
	nextFallback := make(map[string]int)
	lastParent := ""
	nextParentOrder := 0

	addDesc := func(desc collectionDescriptor, order int) {
		if desc.id == "" || seen[desc.id] {
			return
		}
		if desc.name == "" {
			desc.name = uiutil.FormattedCollectionName(desc.id)
		}
		orders = append(orders, orderedDesc{desc: desc, order: order})
		seen[desc.id] = true
	}

	childOrder := func(parent string, weight int) int {
		base, ok := parentOrder[parent]
		if !ok {
			return nextParentOrder + weight
		}
		if weight <= 0 {
			nextFallback[parent]++
			weight = fallbackStart + nextFallback[parent]
		}
		return base + weight
	}

	items := m.colList.Items()
	for _, it := range items {
		ci, ok := it.(index.CollectionItem)
		if !ok {
			lastParent = ""
			continue
		}
		if ci.Resolved == index.TrackingGroupKey {
			lastParent = ""
			continue
		}

		id := ci.Resolved
		if id == "" {
			id = ci.Name
		}

		if ci.Indent {
			if lastParent == "" {
				continue
			}
			childID := ci.Resolved
			if childID == "" {
				childID = fmt.Sprintf("%s/%s", lastParent, ci.Name)
			}
			weight := uiutil.ParseDayNumber(lastParent, ci.Name)
			order := childOrder(lastParent, weight)
			addDesc(collectionDescriptor{id: childID, name: ci.Name, resolved: ci.Resolved}, order)
			continue
		}

		lastParent = id
		parentOrder[id] = nextParentOrder
		nextFallback[id] = 0
		addDesc(collectionDescriptor{id: id, name: ci.Name, resolved: ci.Resolved}, nextParentOrder)
		nextParentOrder += parentStep

		// Month children from calendar state
		if st, ok := m.indexState.Months[id]; ok && st != nil && len(st.Children) > 0 {
			children := make([]index.CollectionItem, len(st.Children))
			copy(children, st.Children)
			sort.SliceStable(children, func(i, j int) bool {
				ti := uiutil.ParseDay(id, children[i].Name)
				tj := uiutil.ParseDay(id, children[j].Name)
				if ti.IsZero() || tj.IsZero() {
					return strings.Compare(children[i].Name, children[j].Name) < 0
				}
				return ti.Before(tj)
			})
			for _, child := range children {
				childID := child.Resolved
				if childID == "" {
					childID = fmt.Sprintf("%s/%s", id, child.Name)
				}
				order := childOrder(id, uiutil.ParseDayNumber(id, child.Name))
				addDesc(collectionDescriptor{id: childID, name: child.Name, resolved: child.Resolved}, order)
			}
		}
	}

	ensureDesc := func(id, name, resolved string) {
		if id == "" || seen[id] {
			return
		}
		if parent, child := splitParentChild(id); parent != "" {
			if _, ok := parentOrder[parent]; ok {
				order := childOrder(parent, uiutil.ParseDayNumber(parent, child))
				addDesc(collectionDescriptor{id: id, name: name, resolved: resolved}, order)
				return
			}
		}
		addDesc(collectionDescriptor{id: id, name: name, resolved: resolved}, nextParentOrder)
		nextParentOrder += parentStep
	}

	focus := m.selectedCollection()
	if m.detailState != nil {
		if active := m.detailState.ActiveCollectionID(); active != "" {
			focus = active
		}
		for _, sec := range m.detailState.Sections() {
			ensureDesc(sec.CollectionID, sec.CollectionName, sec.ResolvedName)
		}
	}
	if focus != "" {
		ensureDesc(focus, uiutil.FriendlyCollectionName(focus), focus)
	}

	sort.SliceStable(orders, func(i, j int) bool {
		return orders[i].order < orders[j].order
	})

	result := make([]collectionDescriptor, len(orders))
	for i := range orders {
		result[i] = orders[i].desc
	}
	return result
}

func (m *Model) descriptorForCollection(id string) collectionDescriptor {
	return collectionDescriptor{
		id:       id,
		name:     uiutil.FriendlyCollectionName(id),
		resolved: id,
	}
}

func splitParentChild(id string) (parent, child string) {
	if strings.Contains(id, "/") {
		parts := strings.SplitN(id, "/", 2)
		parent = parts[0]
		child = parts[1]
	}
	return parent, child
}

func placeholderDetail(focused bool) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	if focused {
		style = style.Foreground(lipgloss.Color("213"))
	}
	return style.Render("<empty>")
}

func (m *Model) startDeleteConfirm(entry *entry.Entry, cmds *[]tea.Cmd) {
	if entry == nil {
		return
	}
	m.confirmAction = confirmDeleteEntry
	m.confirmTargetID = entry.ID
	m.confirmTargetCollection = ""
	m.input.Placeholder = "type yes to delete"
	m.input.SetValue("")
	m.input.CursorEnd()
	m.setMode(modeConfirm)
	if cmd := m.input.Focus(); cmd != nil {
		*cmds = append(*cmds, cmd)
	}
	*cmds = append(*cmds, textinput.Blink)
	m.bottom.UpdateCommandInput("", "")
	m.updateBottomContext()
}

func (m *Model) startCollectionDeleteConfirm(collection string, cmds *[]tea.Cmd) {
	trimmed := strings.TrimSpace(collection)
	if trimmed == "" {
		m.setStatus("Delete collection: select a collection first")
		return
	}
	if trimmed == index.TrackingGroupKey {
		m.setStatus("Delete collection: invalid target")
		return
	}
	if m.svc == nil {
		m.setStatus("Delete collection: service unavailable")
		return
	}
	m.confirmAction = confirmDeleteCollection
	m.confirmTargetCollection = trimmed
	m.confirmTargetID = ""
	m.input.Placeholder = "type yes to delete"
	m.input.SetValue("")
	m.input.CursorEnd()
	m.setMode(modeConfirm)
	if cmd := m.input.Focus(); cmd != nil && cmds != nil {
		*cmds = append(*cmds, cmd)
	}
	if cmds != nil {
		*cmds = append(*cmds, textinput.Blink)
	}
	m.bottom.UpdateCommandInput("", "")
	m.updateBottomContext()
	m.setStatus(fmt.Sprintf("Delete collection %s? type yes to confirm", uiutil.FormattedCollectionName(trimmed)))
}

func (m *Model) cancelConfirm() {
	m.confirmAction = confirmNone
	m.confirmTargetID = ""
	m.confirmTargetCollection = ""
	m.input.Reset()
	m.input.Blur()
	m.setOverlayReserve(0)
	m.setMode(modeNormal)
	m.updateBottomContext()
}

func (m *Model) applyDelete(cmds *[]tea.Cmd, id string) {
	if id == "" {
		m.cancelConfirm()
		return
	}
	entry := m.findEntryByID(id)
	collection := ""
	if entry != nil {
		collection = entry.Collection
	}
	if m.svc == nil {
		*cmds = append(*cmds, func() tea.Msg { return errMsg{errors.New("service unavailable")} })
		m.cancelConfirm()
		return
	}
	if err := m.svc.Delete(m.ctx, id); err != nil {
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		m.cancelConfirm()
		return
	}
	m.setStatus("Deleted")
	if m.panelEntryID == id {
		m.closePanel()
	}
	m.cancelConfirm()
	if collection != "" {
		m.pendingResolved = collection
	}
	*cmds = append(*cmds, m.refreshAll())
}

func (m *Model) applyDeleteCollection(cmds *[]tea.Cmd, collection string) {
	trimmed := strings.TrimSpace(collection)
	if trimmed == "" {
		m.cancelConfirm()
		return
	}
	if m.svc == nil {
		if cmds != nil {
			*cmds = append(*cmds, func() tea.Msg { return errMsg{errors.New("service unavailable")} })
		}
		m.cancelConfirm()
		return
	}
	if err := m.svc.DeleteCollection(m.ctx, trimmed); err != nil {
		if cmds != nil {
			*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		}
		m.cancelConfirm()
		return
	}
	parent := uiutil.ParentCollectionName(trimmed)
	if parent != "" {
		m.pendingResolved = parent
	} else {
		m.pendingResolved = ""
	}
	m.setStatus(fmt.Sprintf("Deleted %s", uiutil.FormattedCollectionName(trimmed)))
	m.cancelConfirm()
	if cmds != nil {
		*cmds = append(*cmds, m.refreshAll())
	}
}

func (m *Model) entriesForCollection(collection string) ([]*entry.Entry, error) {
	if collection == "" {
		return nil, nil
	}
	m.entriesMu.RLock()
	entries, ok := m.entriesCache[collection]
	m.entriesMu.RUnlock()
	if ok {
		return entries, nil
	}
	entries, err := m.svc.Entries(m.ctx, collection)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Created.Before(entries[j].Created.Time)
	})
	entries = dedupeEntriesByID(entries)
	m.entriesMu.Lock()
	m.entriesCache[collection] = entries
	m.entriesMu.Unlock()
	return entries, nil
}

func dedupeEntriesByID(in []*entry.Entry) []*entry.Entry {
	if len(in) <= 1 {
		return in
	}
	result := make([]*entry.Entry, 0, len(in))
	seen := make(map[string]int, len(in))
	for _, e := range in {
		if e == nil {
			continue
		}
		id := e.ID
		if id == "" {
			result = append(result, e)
			continue
		}
		if idx, exists := seen[id]; exists {
			result[idx] = e
			continue
		}
		seen[id] = len(result)
		result = append(result, e)
	}
	if len(result) == len(in) {
		return result
	}
	return result
}

func isMovedImmutable(e *entry.Entry) bool {
	if e == nil || !e.Immutable {
		return false
	}
	return e.Bullet == glyph.MovedCollection || e.Bullet == glyph.MovedFuture
}

func (m *Model) filterEntriesForDisplay(entries []*entry.Entry) ([]*entry.Entry, bool) {
	if len(entries) == 0 {
		return nil, false
	}
	filtered := make([]*entry.Entry, 0, len(entries))
	hasVisible := false
	for _, e := range entries {
		if e == nil {
			continue
		}
		if !m.showHiddenMoved && isMovedImmutable(e) {
			continue
		}
		hasVisible = true
		filtered = append(filtered, e)
	}
	return filtered, hasVisible
}

func visibilityLabel(show bool) string {
	if show {
		return "visible"
	}
	return "hidden"
}

func (m *Model) buildVisibleCollections() map[string]bool {
	visible := make(map[string]bool)
	if m.detailState == nil {
		return visible
	}
	for _, sec := range m.detailState.Sections() {
		if len(sec.Entries) > 0 {
			visible[sec.CollectionID] = true
		}
	}
	return visible
}

func (m *Model) sweepCollections(visible map[string]bool, cmds *[]tea.Cmd, setStatus bool) int {
	if m.showHiddenMoved {
		if setStatus {
			m.setStatus("Sweep: hidden originals currently visible (:show-hidden off to hide)")
		}
		return 0
	}
	if len(visible) == 0 {
		visible = m.buildVisibleCollections()
	}
	items := m.colList.Items()
	if len(items) == 0 {
		if setStatus {
			m.setStatus("Sweep: no collections to prune")
		}
		return 0
	}
	updated := make([]list.Item, 0, len(items))
	removed := 0
	for _, it := range items {
		switch v := it.(type) {
		case index.CollectionItem:
			resolved := v.Resolved
			if resolved == "" {
				resolved = v.Name
			}
			isLeaf := v.Indent || !v.HasChildren
			if isLeaf && !visible[resolved] {
				removed++
				continue
			}
		}
		updated = append(updated, it)
	}
	if removed == 0 {
		if setStatus {
			m.setStatus("Sweep: nothing to hide")
		}
		return 0
	}

	current := m.selectedCollection()
	m.colList.SetItems(updated)
	if len(updated) > 0 {
		idx := indexForResolved(updated, current)
		if idx < 0 {
			idx = 0
		}
		if idx >= 0 && idx < len(updated) {
			m.colList.Select(idx)
		}
		m.updateActiveMonthFromSelection(false, cmds)
	}
	if cmd := m.syncCollectionIndicators(); cmd != nil {
		*cmds = append(*cmds, cmd)
	}
	if setStatus {
		m.setStatus(fmt.Sprintf("Sweep: hid %d collection(s)", removed))
	}
	return removed
}

func (m *Model) handleSweepCommand(cmds *[]tea.Cmd) {
	if m.svc == nil {
		m.setStatus("Sweep: service unavailable")
		return
	}
	m.sweepCollections(nil, cmds, true)
}

func (m *Model) handleImmutableError(err error) bool {
	if errors.Is(err, app.ErrImmutable) {
		m.setStatus("Entry is locked; use :unlock to modify")
		return true
	}
	return false
}

func (m *Model) showHelpPanel() {
	m.helpLines = buildHelpLines()
	m.input.Blur()
	m.setMode(modeHelp)
	m.setOverlayReserve(len(m.helpLines) + 6)
	m.setStatus("Help · esc to close")
}

func buildHelpLines() []string {
	return []string{
		"Navigation:",
		"  ←/→ switch panes · ↑/↓ move · gg/G top/bottom · [/] fold months",
		"  Enter activate selection · Esc cancel current mode · ? open help",
		"",
		"Entries:",
		"  o add entry · O add child · i edit · x complete · dd strike",
		"  > move to collection · < migrate to Future",
		"",
		"Bullets & Signifiers:",
		"  b open bullet/signifier menu · v focus signifier options",
		"  Enter applies selection · Tab switches between sections",
		"",
		"Command Mode (:) :",
		"  :mkdir parent/child create collections",
		"  :show-hidden toggle moved originals · :today jump to Today · :future jump to Future",
		"  :delete-collection remove a collection (confirmation required)",
		"  :report window show completed entries",
		"  :help open this guide · :q quit the UI",
	}
}

func todayLabels() (month string, day string, resolved string) {
	now := time.Now()
	month = now.Format("January 2006")
	day = now.Format("January 2, 2006")
	resolved = fmt.Sprintf("%s/%s", month, day)
	return
}

func todayResolvedCollection() string {
	_, _, resolved := todayLabels()
	return resolved
}

func relativeTime(then time.Time, now time.Time) string {
	if then.IsZero() {
		return "unknown"
	}
	delta := now.Sub(then)
	direction := "ago"
	if delta < 0 {
		delta = -delta
		direction = "from now"
	}
	switch {
	case delta >= 24*time.Hour:
		days := int(delta.Hours() / 24)
		if days == 1 {
			return "1 day " + direction
		}
		return fmt.Sprintf("%d days %s", days, direction)
	case delta >= time.Hour:
		hours := int(delta.Hours())
		if hours == 1 {
			return "1 hour " + direction
		}
		return fmt.Sprintf("%d hours %s", hours, direction)
	case delta >= time.Minute:
		mins := int(delta.Minutes())
		if mins == 1 {
			return "1 minute " + direction
		}
		return fmt.Sprintf("%d minutes %s", mins, direction)
	default:
		secs := int(delta.Seconds())
		if secs <= 1 {
			return "just now"
		}
		return fmt.Sprintf("%d seconds %s", secs, direction)
	}
}

func (m *Model) updateMoveSuggestions(value string) {
	options := m.buildMoveOptions(value)
	m.bottom.SetCommandDefinitions(options)
	m.bottom.UpdateCommandInput(m.input.Value(), m.input.View())
}

func (m *Model) buildMoveOptions(value string) []bottombar.CommandOption {
	trimmed := strings.TrimSpace(value)
	prefix, current := splitMoveInput(trimmed)
	base := strings.Join(prefix, "/")
	lowerCurrent := strings.ToLower(current)
	lowerTrimmed := strings.ToLower(trimmed)
	seen := make(map[string]struct{})
	candidates := make([]string, 0)
	for _, col := range m.moveCollections {
		col = strings.TrimSpace(col)
		if col == "" {
			continue
		}
		parts := strings.Split(col, "/")
		if len(prefix) > len(parts) {
			continue
		}
		match := true
		for i, seg := range prefix {
			if parts[i] != seg {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		var candidate string
		if len(prefix) == len(parts) {
			candidate = strings.Join(parts, "/")
			if trimmed != "" && !strings.HasPrefix(strings.ToLower(candidate), lowerTrimmed) {
				continue
			}
		} else {
			next := parts[len(prefix)]
			if lowerCurrent != "" && !strings.HasPrefix(strings.ToLower(next), lowerCurrent) {
				continue
			}
			if base == "" {
				candidate = next
			} else {
				candidate = base + "/" + next
			}
		}
		if !m.collectionExists(candidate) && m.collectionHasChildren(candidate) && !strings.HasSuffix(candidate, "/") {
			candidate += "/"
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
	}

	monthlyParent := false
	if base != "" {
		baseType := m.lookupCollectionType(base)
		if baseType == collection.TypeMonthly || strings.EqualFold(base, "Future") {
			monthlyParent = true
		}
	}
	if monthlyParent {
		now := time.Now()
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		for i := 0; i < 12; i++ {
			monthTime := start.AddDate(0, i, 0)
			label := monthTime.Format("January 2006")
			if lowerCurrent != "" && !strings.HasPrefix(strings.ToLower(label), lowerCurrent) {
				continue
			}
			candidate := label
			if base != "" {
				candidate = base + "/" + label
			}
			if trimmed != "" && base == "" && !strings.HasPrefix(strings.ToLower(candidate), lowerTrimmed) {
				continue
			}
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			candidates = append(candidates, candidate)
		}
	}
	sort.Strings(candidates)
	options := make([]bottombar.CommandOption, len(candidates))
	for i, cand := range candidates {
		options[i] = bottombar.CommandOption{Name: cand}
	}
	return options
}

func splitMoveInput(trimmed string) ([]string, string) {
	if trimmed == "" {
		return nil, ""
	}
	parts := strings.Split(trimmed, "/")
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		clean = append(clean, strings.TrimSpace(p))
	}
	hasTrailing := strings.HasSuffix(trimmed, "/")
	if hasTrailing {
		if len(clean) > 0 && clean[len(clean)-1] == "" {
			clean = clean[:len(clean)-1]
		}
		return clean, ""
	}
	if len(clean) == 0 {
		return nil, ""
	}
	prefix := clean[:len(clean)-1]
	current := clean[len(clean)-1]
	return prefix, current
}

func (m *Model) collectionHasChildren(path string) bool {
	prefix := strings.TrimSpace(path)
	if prefix == "" {
		return false
	}
	prefix += "/"
	for _, col := range m.moveCollections {
		if strings.HasPrefix(col, prefix) {
			return true
		}
	}
	return false
}

func (m *Model) collectionExists(path string) bool {
	for _, col := range m.moveCollections {
		if strings.EqualFold(strings.TrimSpace(col), strings.TrimSpace(path)) {
			return true
		}
	}
	return false
}

func (m *Model) handleReportKey(msg tea.KeyPressMsg) bool {
	if m.report == nil {
		return false
	}
	switch msg.String() {
	case "esc", "q":
		m.exitReportMode()
		return true
	case "j", "down":
		m.report.ScrollLines(1)
	case "k", "up":
		m.report.ScrollLines(-1)
	case "pgdown", "space", "ctrl+f":
		m.report.ScrollPages(1)
	case "pgup", "ctrl+b":
		m.report.ScrollPages(-1)
	case "g", "home":
		m.report.ScrollHome()
	case "G", "end":
		m.report.ScrollEnd()
	default:
		return false
	}
	return true
}

func (m *Model) exitReportMode() {
	if m.report != nil {
		m.report.Clear()
	}
	m.setMode(modeNormal)
	m.setOverlayReserve(0)
	m.setStatus("Report closed")
}

func (m *Model) handleMigrationKey(msg tea.KeyPressMsg, cmds *[]tea.Cmd) bool {
	if m.migration == nil || !m.migration.Active {
		return false
	}
	key := msg.String()
	switch key {
	case "esc":
		if m.migration.Focus == migrationview.FocusTargets {
			m.migration.Focus = migrationview.FocusTasks
			m.updateBottomContext()
		} else {
			m.exitMigrationMode(cmds)
		}
		return true
	case "down", "j":
		if m.migration.Focus == migrationview.FocusTasks {
			if len(m.migration.Items) == 0 {
				return true
			}
			if m.migration.Index < len(m.migration.Items)-1 {
				m.migration.Index++
			} else {
				m.migration.Index = len(m.migration.Items) - 1
			}
			m.migration.EnsureTaskVisible()
		} else {
			if len(m.migration.Targets) == 0 {
				return true
			}
			if m.migration.TargetIndex < len(m.migration.Targets)-1 {
				m.migration.TargetIndex++
			}
			m.migration.EnsureTargetVisible()
		}
		return true
	case "up", "k":
		if m.migration.Focus == migrationview.FocusTasks {
			if len(m.migration.Items) == 0 {
				return true
			}
			if m.migration.Index > 0 {
				m.migration.Index--
			} else {
				m.migration.Index = 0
			}
			m.migration.EnsureTaskVisible()
		} else {
			if len(m.migration.Targets) == 0 {
				return true
			}
			if m.migration.TargetIndex > 0 {
				m.migration.TargetIndex--
			}
			m.migration.EnsureTargetVisible()
		}
		return true
	case ">":
		if m.migration.Focus == migrationview.FocusTasks {
			if len(m.migration.Items) == 0 {
				m.setStatus("No task selected")
				return true
			}
			if len(m.migration.Targets) == 0 {
				m.setStatus("Migration: no collections available")
				return true
			}
			m.migration.Focus = migrationview.FocusTargets
			m.migration.EnsureTargetVisible()
			m.updateBottomContext()
		} else {
			m.performMigrationMove(cmds)
		}
		return true
	case "enter":
		if m.migration.Focus == migrationview.FocusTargets {
			m.performMigrationMove(cmds)
			return true
		}
		return false
	case "<":
		if item := m.migration.CurrentItem(); item != nil && item.Entry != nil {
			m.applyMoveToFuture(cmds, item.Entry.ID)
			m.migration.Focus = migrationview.FocusTasks
			m.updateBottomContext()
		} else {
			m.setStatus("No task selected")
		}
		return true
	case "x":
		if m.migration.Focus == migrationview.FocusTasks {
			if item := m.migration.CurrentItem(); item != nil && item.Entry != nil {
				m.applyComplete(cmds, item.Entry)
			} else {
				m.setStatus("No task selected")
			}
		}
		return true
	case "delete":
		if m.migration.Focus == migrationview.FocusTasks {
			if item := m.migration.CurrentItem(); item != nil && item.Entry != nil {
				m.applyStrikeEntry(cmds, item.Entry.ID)
			} else {
				m.setStatus("No task selected")
			}
		}
		return true
	default:
		return false
	}
}

func (m *Model) startMigrationMode(candidates []app.MigrationCandidate, label string, since, until time.Time) {
	if m.migration == nil {
		m.migration = migrationview.New(m.theme, relativeTime)
	}
	items := make([]migrationview.Item, 0, len(candidates))
	for _, cand := range candidates {
		if cand.Entry == nil {
			continue
		}
		items = append(items, migrationview.Item{
			Entry:       cand.Entry,
			Parent:      cand.Parent,
			LastTouched: cand.LastTouched,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		c1 := strings.ToLower(strings.TrimSpace(items[i].Entry.Collection))
		c2 := strings.ToLower(strings.TrimSpace(items[j].Entry.Collection))
		if c1 != c2 {
			return c1 < c2
		}
		return items[i].LastTouched.After(items[j].LastTouched)
	})

	targets, targetMetas, monthChildren, err := m.buildMigrationTargets()
	if err != nil {
		m.setStatus("Migration: " + err.Error())
		return
	}
	m.migration.Active = true
	m.migration.Label = label
	m.migration.Since = since
	m.migration.Until = until
	m.migration.Items = items
	m.migration.Index = 0
	m.migration.Scroll = 0
	m.migration.Targets = targets
	m.migration.TargetMetas = targetMetas
	m.migration.MonthChildren = monthChildren
	m.migration.TargetIndex = 0
	m.migration.TargetScroll = 0
	m.migration.Focus = migrationview.FocusTasks
	m.migration.MigratedCount = 0
	m.updateMigrationViewport()
	m.setMode(modeMigration)
	if m.migration != nil {
		m.migration.EnsureTaskVisible()
		m.migration.EnsureTargetVisible()
	}
	m.setStatus(fmt.Sprintf("Migration started · last %s (%d open)", label, len(items)))
	m.updateBottomContext()
}

func (m *Model) buildMigrationTargets() ([]string, map[string]collection.Meta, map[string][]string, error) {
	if m.svc == nil {
		return nil, nil, nil, errors.New("service unavailable")
	}
	metas, err := m.svc.CollectionsMeta(m.ctx, "")
	if err != nil {
		return nil, nil, nil, err
	}
	metaMap := make(map[string]collection.Meta, len(metas))
	children := make(map[string][]string)
	order := make([]string, 0, len(metas))
	for _, meta := range metas {
		name := strings.TrimSpace(meta.Name)
		if name == "" {
			continue
		}
		if _, ok := metaMap[name]; ok {
			continue
		}
		metaMap[name] = meta
		order = append(order, name)
		if parent := uiutil.ParentCollectionName(name); parent != "" {
			children[parent] = append(children[parent], name)
		}
	}
	sort.Strings(order)
	for parent := range children {
		sort.Strings(children[parent])
	}
	return order, metaMap, children, nil
}

func (m *Model) exitMigrationMode(cmds *[]tea.Cmd) {
	count := 0
	if m.migration != nil {
		count = m.migration.MigratedCount
	}
	m.migration = migrationview.New(m.theme, relativeTime)
	m.setMode(modeNormal)
	m.setOverlayReserve(0)
	m.applyReserve()
	if count > 0 {
		m.setStatus(fmt.Sprintf("Migration finished · %d items updated", count))
	} else {
		m.setStatus("Migration ended")
	}
	if cmds != nil {
		*cmds = append(*cmds, m.refreshAll())
	}
}

func (m *Model) handleMigrationAfterAction(originalID string, result *entry.Entry) {
	if m.migration == nil || !m.migration.Active {
		return
	}
	if !m.migration.RemoveItem(originalID) {
		return
	}
	m.migration.MigratedCount++
	m.migration.Focus = migrationview.FocusTasks
	if len(m.migration.Items) == 0 {
		m.updateBottomContext()
		m.setStatus(fmt.Sprintf("Migration complete · %d items updated", m.migration.MigratedCount))
		return
	}
	m.migration.EnsureTaskVisible()
	m.updateBottomContext()
}

func (m *Model) performMigrationMove(cmds *[]tea.Cmd) {
	if m.migration == nil {
		m.setStatus("No migration in progress")
		return
	}
	item := m.migration.CurrentItem()
	if item == nil || item.Entry == nil {
		m.setStatus("No task selected")
		return
	}
	target := m.migration.CurrentTarget()
	if strings.TrimSpace(target) == "" {
		m.setStatus("Select a collection target")
		return
	}
	m.applyMove(cmds, item.Entry.ID, target)
	m.migration.Focus = migrationview.FocusTasks
	m.migration.EnsureTaskVisible()
	m.updateBottomContext()
}

func (m *Model) collectionPaneWidth() int {
	left := m.termWidth / 3
	if left < 24 {
		left = 24
	}
	if left > 40 {
		left = 40
	}
	if left <= 0 {
		left = 24
	}
	return left
}

func (m *Model) openTaskPanel(entry *entry.Entry) {
	if entry == nil {
		return
	}
	m.panelEntryID = entry.ID
	m.panelCollection = entry.Collection
	m.populateTaskPanel(entry)
	m.setMode(modePanel)
}

func (m *Model) populateTaskPanel(entry *entry.Entry) {
	if entry == nil {
		return
	}
	m.panelModel.SetContent("Task Detail", taskPanelLines(entry))
	_, height := m.panelModel.View()
	m.setOverlayReserve(height)
}

func (m *Model) closePanel() {
	m.panelModel.Reset()
	m.panelEntryID = ""
	m.panelCollection = ""
	m.setOverlayReserve(0)
	if m.mode == modePanel {
		m.setMode(modeNormal)
	}
}

func (m *Model) beginEdit(entry *entry.Entry, cmds *[]tea.Cmd) {
	if entry == nil {
		return
	}
	m.mode = modeInsert
	m.action = actionEdit
	m.input.Placeholder = "Edit message"
	m.input.SetValue(entry.Message)
	m.input.CursorEnd()
	if cmd := m.input.Focus(); cmd != nil {
		*cmds = append(*cmds, cmd)
	}
	*cmds = append(*cmds, textinput.Blink)
	m.updateBottomContext()
}

func taskPanelLines(e *entry.Entry) []string {
	var lines []string
	collection := e.Collection
	if collection == "" {
		collection = "<unspecified>"
	}
	lines = append(lines, fmt.Sprintf("Collection: %s", collection))
	if !e.Created.IsZero() {
		lines = append(lines, fmt.Sprintf("Created: %s", e.Created.Format(time.RFC3339)))
	}
	bullet := e.Bullet.Glyph()
	lines = append(lines, fmt.Sprintf("Bullet: %s (%s)", bullet.Symbol, bullet.Meaning))
	if e.Signifier == glyph.None {
		lines = append(lines, "Signifier: none")
	} else {
		sig := e.Signifier.Glyph()
		lines = append(lines, fmt.Sprintf("Signifier: %s (%s)", sig.Symbol, sig.Meaning))
	}
	id := e.ID
	if id == "" {
		id = "<pending>"
	}
	lines = append(lines, fmt.Sprintf("ID: %s", id))
	if e.ParentID != "" {
		lines = append(lines, fmt.Sprintf("Parent: %s", e.ParentID))
	}
	if e.On != nil && !e.On.IsZero() {
		lines = append(lines, fmt.Sprintf("Scheduled: %s", e.On.Format(time.RFC3339)))
	}
	lines = append(lines, "")
	lines = append(lines, "Message:")
	if strings.TrimSpace(e.Message) == "" {
		lines = append(lines, "  <empty>")
	} else {
		for _, msgLine := range strings.Split(e.Message, "\n") {
			lines = append(lines, "  "+msgLine)
		}
	}
	if len(e.History) > 0 {
		lines = append(lines, "")
		lines = append(lines, "History:")
		for _, record := range e.History {
			lines = append(lines, "  "+formatHistoryRecord(record))
		}
	}
	lines = append(lines, "")
	lines = append(lines, "Actions: enter/esc close · e edit · b bullet/signifier menu · v signifier menu")
	return lines
}

func formatHistoryRecord(r entry.HistoryRecord) string {
	ts := r.Timestamp.Time
	var tsString string
	if ts.IsZero() {
		tsString = "(unknown time)"
	} else {
		tsString = ts.Format("2006-01-02 15:04")
	}
	switch r.Action {
	case entry.HistoryActionAdded:
		if r.To == "" {
			return fmt.Sprintf("%s · created", tsString)
		}
		return fmt.Sprintf("%s · added to %s", tsString, r.To)
	case entry.HistoryActionMoved:
		from := r.From
		if from == "" {
			from = "(unknown)"
		}
		to := r.To
		if to == "" {
			to = "(unknown)"
		}
		return fmt.Sprintf("%s · moved %s → %s", tsString, from, to)
	case entry.HistoryActionCompleted:
		return fmt.Sprintf("%s · completed", tsString)
	case entry.HistoryActionStruck:
		return fmt.Sprintf("%s · struck out", tsString)
	default:
		if r.To != "" || r.From != "" {
			return fmt.Sprintf("%s · %s (%s → %s)", tsString, r.Action, r.From, r.To)
		}
		return fmt.Sprintf("%s · %s", tsString, r.Action)
	}
}

func (m *Model) findEntryByID(id string) *entry.Entry {
	if m.detailState == nil || id == "" {
		return nil
	}
	for _, sec := range m.detailState.Sections() {
		for _, e := range sec.Entries {
			if e.ID == id {
				return e
			}
		}
	}
	return nil
}

func (m *Model) ensureCollectionSelection(direction int) {
	idx := m.colList.Index()
	items := m.colList.Items()
	if idx < 0 || idx >= len(items) {
		return
	}
	if direction >= 0 {
		for idx < len(items) && isCalendarHeader(items[idx]) {
			m.colList.CursorDown()
			idx = m.colList.Index()
		}
	} else {
		for idx >= 0 && isCalendarHeader(items[idx]) {
			m.colList.CursorUp()
			idx = m.colList.Index()
		}
	}
}

func isCalendarHeader(it list.Item) bool {
	_, ok := it.(*index.CalendarHeaderItem)
	return ok
}

func calendarRows(state *index.MonthState) []*index.CalendarRowItem {
	if state == nil || state.Calendar == nil {
		return nil
	}
	return state.Calendar.Rows()
}

func (m *Model) markCalendarSelection() tea.Cmd {
	sel := m.colList.SelectedItem()
	switch v := sel.(type) {
case *index.CalendarHeaderItem:
	state := m.indexState.Months[v.Month]
	rows := calendarRows(state)
	if len(rows) == 0 {
		return nil
	}
	m.colList.Select(rows[0].RowIndex)
	return m.markCalendarSelection()
	case *index.CalendarRowItem:
		state := m.indexState.Months[v.Month]
		if state == nil {
			return nil
		}
		day := m.indexState.Selection[v.Month]
		if day == 0 || !index.ContainsDay(v.Days, day) {
			day = index.FirstNonZero(v.Days)
		}
		if day == 0 {
			return nil
		}
		m.indexState.Selection[v.Month] = day
		m.pendingResolved = index.FormatDayPath(state.MonthTime, day)
		var cmds []tea.Cmd
		m.applyActiveCalendarMonth(v.Month, true, &cmds)
		cmds = append(cmds, m.loadDetailSectionsWithFocus(m.pendingResolved, ""))
		return tea.Batch(cmds...)
	default:
		return nil
	}
}

func (m *Model) moveCalendarCursor(dx, dy int) tea.Cmd {
	item := m.colList.SelectedItem()
	var month string
	switch v := item.(type) {
	case *index.CalendarRowItem:
		month = v.Month
	case *index.CalendarHeaderItem:
		month = v.Month
	default:
		return nil
	}

state := m.indexState.Months[month]
	rows := calendarRows(state)
	if len(rows) == 0 {
		return nil
	}

	selected := m.indexState.Selection[month]
	if selected == 0 {
		selected = index.DefaultSelectedDay(month, state.MonthTime, state.Children, m.pendingResolved, time.Now())
		if selected == 0 && len(rows) > 0 {
			selected = index.FirstNonZero(rows[0].Days)
		}
	}
	if selected == 0 {
		return nil
	}

	newDay := selected + dx + dy*7
	daysInMonth := index.DaysIn(state.MonthTime)
	if newDay < 1 {
		newDay = 1
	}
	if newDay > daysInMonth {
		newDay = daysInMonth
	}
	if newDay == selected {
		return nil
	}

	m.indexState.Selection[month] = newDay
	m.pendingResolved = index.FormatDayPath(state.MonthTime, newDay)
	m.detailRevealTarget = m.pendingResolved

	var cmds []tea.Cmd
	m.applyActiveCalendarMonth(month, true, &cmds)
	if week := m.findWeekForDay(month, newDay); week != nil {
		if m.colList.Index() != week.RowIndex {
			m.colList.Select(week.RowIndex)
		}
	}
	cmds = append(cmds, m.loadDetailSectionsWithFocus(m.pendingResolved, ""))
	return tea.Batch(cmds...)
}

func (m *Model) moveDetailCursor(delta int, cmds *[]tea.Cmd) bool {
	if m.detailState == nil {
		return false
	}
	prevCollection := m.detailState.ActiveCollectionID()
	prevEntry := m.detailState.ActiveEntryID()
	if ok := m.detailState.MoveEntry(delta); !ok {
		if delta > 0 {
			return m.moveDetailSection(1, cmds)
		}
		if delta < 0 {
			return m.moveDetailSection(-1, cmds)
		}
		return false
	}
	currentEntry := m.detailState.ActiveEntryID()
	newCollection := m.detailState.ActiveCollectionID()
	if newCollection != "" && (newCollection != prevCollection || currentEntry != prevEntry) {
		if cmd := detailFocusChangedCmd(newCollection, currentEntry, true); cmd != nil {
			*cmds = append(*cmds, cmd)
		}
	}
	m.updateBottomContext()
	return true
}

func (m *Model) moveDetailSection(delta int, cmds *[]tea.Cmd) bool {
	if m.detailState == nil {
		return false
	}
	prevCollection := m.detailState.ActiveCollectionID()
	if ok := m.detailState.MoveSection(delta); !ok {
		return false
	}
	newCollection := m.detailState.ActiveCollectionID()
	if newCollection == "" {
		return false
	}
	if newCollection != prevCollection {
		if cmd := detailFocusChangedCmd(newCollection, m.detailState.ActiveEntryID(), true); cmd != nil {
			*cmds = append(*cmds, cmd)
		}
	}
	m.updateBottomContext()
	return true
}

func (m *Model) selectCollectionByID(id string, cmds *[]tea.Cmd) bool {
	if id == "" {
		return false
	}
	items := m.colList.Items()
	idx := indexForResolved(items, id)
	if idx == -1 {
		idx = indexForName(items, id)
	}
	if idx == -1 {
		return false
	}
	if idx != m.colList.Index() {
		m.colList.Select(idx)
		m.updateActiveMonthFromSelection(false, cmds)
		if cmd := m.syncCollectionIndicators(); cmd != nil {
			*cmds = append(*cmds, cmd)
		}
	}
	return true
}

func (m *Model) findWeekForDay(month string, day int) *index.CalendarRowItem {
	state := m.indexState.Months[month]
	if state == nil {
		return nil
	}
	for _, week := range calendarRows(state) {
		if index.ContainsDay(week.Days, day) {
			return week
		}
	}
	return nil
}

func (m *Model) alignCollectionSelection(resolved string, cmds *[]tea.Cmd) {
	items := m.colList.Items()
	if len(items) == 0 {
		return
	}
	idx := indexForResolved(items, resolved)
	if idx == -1 {
		idx = indexForName(items, resolved)
	}
	if idx == -1 || idx == m.colList.Index() {
		return
	}
	m.colList.Select(idx)
	m.updateActiveMonthFromSelection(false, cmds)
	if _, ok := items[idx].(*index.CalendarRowItem); ok {
		if day := index.DayFromPath(resolved); day > 0 {
			month := resolved
			if i := strings.IndexRune(resolved, '/'); i >= 0 {
				month = resolved[:i]
			}
			m.indexState.Selection[month] = day
		}
		m.pendingResolved = resolved
		if cmd := m.markCalendarSelection(); cmd != nil {
			*cmds = append(*cmds, cmd)
		}
	}
	if cmd := m.syncCollectionIndicators(); cmd != nil {
		*cmds = append(*cmds, cmd)
	}
}

func (m *Model) updateCalendarSelection(resolved string, cmds *[]tea.Cmd) {
	if resolved == "" {
		return
	}
	day := index.DayFromPath(resolved)
	if day == 0 {
		return
	}
	month := resolved
	if idx := strings.IndexRune(resolved, '/'); idx >= 0 {
		month = resolved[:idx]
	}
	if month == "" {
		return
	}
	if m.indexState.Selection == nil {
		m.indexState.Selection = make(map[string]int)
	}
	changed := m.indexState.Selection[month] != day
	m.indexState.Selection[month] = day
	if m.indexState.ActiveMonthKey != month {
		m.indexState.ActiveMonthKey = month
		changed = true
	}
	m.applyActiveCalendarMonth(month, changed, cmds)
}

func (m *Model) refreshCalendarMonth(month string) tea.Cmd {
	state := m.indexState.Months[month]
	if state == nil || state.HeaderIdx < 0 {
		return nil
	}
	selected := m.indexState.Selection[month]
	if m.indexState.ActiveMonthKey != month {
		selected = 0
	}
	if state.Calendar == nil {
		state.Calendar = index.NewCalendarModel(month, selected, time.Now())
		state.Calendar.SetChildren(state.Children)
	}
	oldRows := append([]*index.CalendarRowItem(nil), calendarRows(state)...)
	state.Calendar.SetNow(time.Now())
	state.Calendar.SetChildren(state.Children)
	state.Calendar.SetSelected(selected)
	header := state.Calendar.Header()
	weeks := state.Calendar.Rows()
	if header == nil {
		return nil
	}

	var cmds []tea.Cmd
	headerIdx := state.HeaderIdx
	if headerIdx >= len(m.colList.Items()) {
		return nil
	}
	if cmd := m.colList.SetItem(headerIdx, header); cmd != nil {
		cmds = append(cmds, cmd)
	}

	oldCount := len(oldRows)
	newCount := len(weeks)
	rowBase := headerIdx + 1

	if oldCount > newCount {
		for i := oldCount - 1; i >= newCount; i-- {
			m.colList.RemoveItem(rowBase + i)
		}
	} else if oldCount < newCount {
		for i := oldCount; i < newCount; i++ {
			idx := rowBase + i
			if cmd := m.colList.InsertItem(idx, weeks[i]); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	for i := 0; i < newCount; i++ {
		idx := rowBase + i
		if idx >= len(m.colList.Items()) {
			break
		}
		week := weeks[i]
		week.RowIndex = idx
		if cmd := m.colList.SetItem(idx, week); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	m.indexState.Months[month] = state
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) applyActiveCalendarMonth(month string, force bool, cmds *[]tea.Cmd) {
	prev := m.indexState.ActiveMonthKey
	changed := prev != month
	if changed {
		m.indexState.ActiveMonthKey = month
	}
	if month != "" && (force || changed) {
		if cmd := m.refreshCalendarMonth(month); cmd != nil {
			*cmds = append(*cmds, cmd)
		}
	}
	if changed && prev != "" {
		if cmd := m.refreshCalendarMonth(prev); cmd != nil {
			*cmds = append(*cmds, cmd)
		}
	}
}

func (m *Model) updateActiveMonthFromSelection(force bool, cmds *[]tea.Cmd) {
	sel := m.colList.SelectedItem()
	if sel == nil {
		m.applyActiveCalendarMonth("", false, cmds)
		return
	}
	switch v := sel.(type) {
	case *index.CalendarRowItem:
		m.applyActiveCalendarMonth(v.Month, force, cmds)
	case *index.CalendarHeaderItem:
		m.applyActiveCalendarMonth(v.Month, force, cmds)
	default:
		m.applyActiveCalendarMonth("", false, cmds)
	}
}
