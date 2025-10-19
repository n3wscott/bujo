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
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/runner/tea/internal/bottombar"
	"tableflip.dev/bujo/pkg/runner/tea/internal/detailview"
	"tableflip.dev/bujo/pkg/runner/tea/internal/indexview"
	"tableflip.dev/bujo/pkg/runner/tea/internal/panel"
	"tableflip.dev/bujo/pkg/store"
	"tableflip.dev/bujo/pkg/timeutil"
)

// Model states and actions
type mode int

const (
	modeNormal mode = iota
	modeInsert
	modeCommand
	modeHelp
	modeBulletSelect
	modePanel
	modeConfirm
	modeParentSelect
	modeReport
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
	{Name: "help", Description: "Show help guide"},
	{Name: "mkdir", Description: "Create collection (supports hierarchy)"},
	{Name: "show-hidden", Description: "Toggle moved originals visibility"},
	{Name: "lock", Description: "Lock selected entry"},
	{Name: "unlock", Description: "Unlock selected entry"},
	{Name: "delete", Description: "Delete selected entry"},
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

	termWidth            int
	termHeight           int
	verticalReserve      int
	overlayReserve       int
	indexState           *indexview.State
	pendingResolved      string
	detailWidth          int
	detailHeight         int
	detailState          *detailview.State
	entriesCache         map[string][]*entry.Entry
	entriesMu            sync.RWMutex
	detailOrder          []collectionDescriptor
	panelModel           panel.Model
	panelEntryID         string
	panelCollection      string
	confirmAction        confirmAction
	confirmTargetID      string
	watchCh              <-chan store.Event
	watchCancel          context.CancelFunc
	detailRevealTarget   string
	pendingChildParent   string
	parentSelect         parentSelectState
	pendingAddCollection string
	showHiddenMoved      bool

	focusDel list.DefaultDelegate
	blurDel  list.DefaultDelegate

	bottom    bottombar.Model
	helpLines []string

	commandSelectActive  bool
	commandOriginalInput string

	reportSections []detailview.Section
	reportLines    []string
	reportOffset   int
	reportLabel    string
	reportSince    time.Time
	reportUntil    time.Time
	reportTotal    int
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

	bottom := bottombar.New()
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
		indexState:       indexview.NewState(),
		bottom:           bottom,
		resumeMode:       modeNormal,
		detailState:      detailview.NewState(),
		entriesCache:     make(map[string][]*entry.Entry),
		panelModel:       panel.New(),
		bulletMenuFocus:  menuSectionBullet,
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
		cols, err := m.svc.Collections(m.ctx)
		if err != nil {
			return errMsg{err}
		}
		items := m.buildCollectionItems(cols, current, now)
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
	case indexview.CollectionItem:
		if v.Resolved != "" {
			return v.Resolved
		}
		return v.Name
	case *indexview.CalendarRowItem:
		state := m.indexState.Months[v.Month]
		if state == nil {
			return ""
		}
		day := m.indexState.Selection[v.Month]
		if day == 0 || !indexview.ContainsDay(v.Days, day) {
			day = indexview.FirstNonZero(v.Days)
		}
		if day == 0 {
			return ""
		}
		return indexview.FormatDayPath(state.MonthTime, day)
	case *indexview.CalendarHeaderItem:
		return v.Month
	default:
		return ""
	}
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

		sections := make([]detailview.Section, 0, len(order))
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
			if name == "" {
				name = friendlyCollectionName(desc.id)
			}
			sections = append(sections, detailview.Section{
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
				sec detailview.Section
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
	sections         []detailview.Section
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
	case modePanel:
		return m.handlePanelKey(msg, cmds)
	case modeConfirm:
		return m.handleConfirmKey(msg, cmds)
	case modeParentSelect:
		return m.handleParentSelectKey(msg, cmds)
	case modeReport:
		return m.handleReportKey(msg)
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
			}
		} else {
			m.setStatus("Type yes to confirm")
		}
		return true
	case "esc", "q":
		m.cancelConfirm()
		m.setStatus("Delete cancelled")
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
	case "esc", "q":
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
		input := strings.TrimSpace(m.input.Value())
		m.executeCommand(input, cmds)
		return true
	case "esc":
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
		m.bottom.UpdateCommandInput(m.input.Value(), m.input.View())
		m.applyReserve()
		m.commandSelectActive = false
		m.commandOriginalInput = ""
		return false
	}
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
			m.applyComplete(cmds, it.ID)
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
			m.setMode(modeInsert)
			m.action = actionMove
			m.input.Placeholder = "Move to collection"
			m.input.SetValue(it.Collection)
			if cmd := m.input.Focus(); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
			*cmds = append(*cmds, textinput.Blink)
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
	case actionMove:
		if it := m.currentEntry(); it != nil {
			m.applyMove(cmds, it.ID, input)
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
	case "help":
		m.input.Reset()
		m.input.Blur()
		m.bottom.UpdateCommandInput("", "")
		m.showHelpPanel()
		return
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
	segments := strings.Split(name, "/")
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
		m.setStatus("mkdir requires a valid collection path")
		return
	}
	if m.svc == nil {
		*cmds = append(*cmds, func() tea.Msg { return errMsg{errors.New("service unavailable")} })
		return
	}
	if err := m.svc.EnsureCollections(m.ctx, paths); err != nil {
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		m.setStatus("ERR: " + err.Error())
		return
	}
	target := paths[len(paths)-1]
	m.setStatus(fmt.Sprintf("Collection created: %s", target))
	m.pendingResolved = target
	*cmds = append(*cmds, m.loadDetailSectionsWithFocus(target, ""))
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
	m.reportSections = m.buildReportSections(result)
	m.reportLabel = label
	m.reportSince = since
	m.reportUntil = until
	m.reportTotal = result.Total
	m.renderReportLines()
	m.reportOffset = 0
	m.setMode(modeReport)
	m.input.Reset()
	m.input.Blur()
	m.bottom.UpdateCommandInput("", "")
	m.setOverlayReserve(0)
	m.updateBottomContext()
	m.setStatus(fmt.Sprintf("Report · last %s (%d completed)", label, result.Total))
	return nil
}

func (m *Model) buildReportSections(result app.ReportResult) []detailview.Section {
	sections := make([]detailview.Section, 0, len(result.Sections))
	for _, sec := range result.Sections {
		entries := make([]*entry.Entry, len(sec.Entries))
		for i, item := range sec.Entries {
			entries[i] = item.Entry
		}
		name := friendlyCollectionName(sec.Collection)
		sections = append(sections, detailview.Section{
			CollectionID:   sec.Collection,
			CollectionName: name,
			ResolvedName:   sec.Collection,
			Entries:        entries,
		})
	}
	return sections
}

func (m *Model) renderReportLines() {
	header := fmt.Sprintf("Report · last %s (%s → %s)", m.reportLabel, formatReportTime(m.reportSince), formatReportTime(m.reportUntil))
	summary := fmt.Sprintf("%d completed entries", m.reportTotal)
	lines := []string{header, summary, ""}

	if m.reportTotal == 0 {
		m.reportLines = append(lines, "No completed entries found in this window.")
		m.ensureReportBounds()
		return
	}

	state := detailview.NewState()
	state.SetSections(m.reportSections)
	width := m.termWidth - 6
	if width < 20 {
		width = 20
	}
	state.SetWrapWidth(width)
	state.ScrollToTop()
	state.ClearSelection()
	view, _ := state.Viewport(1 << 15)
	if strings.TrimSpace(view) != "" {
		lines = append(lines, strings.Split(view, "\n")...)
	}
	m.reportLines = lines
	m.ensureReportBounds()
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
		if len(m.reportSections) > 0 {
			m.renderReportLines()
		}
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
			if _, ok := m.colList.SelectedItem().(*indexview.CalendarRowItem); ok {
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
		if !m.showHiddenMoved {
			m.pruneHiddenCollections(visibleSet, &cmds)
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
	if id == "" || target == "" {
		return
	}
	if _, err := m.svc.Move(m.ctx, id, target); err != nil {
		if m.handleImmutableError(err) {
			return
		}
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		return
	}
	m.setStatus("Moved")
	*cmds = append(*cmds, m.refreshAll())
}

func (m *Model) applyMoveToFuture(cmds *[]tea.Cmd, id string) {
	if id == "" {
		return
	}
	if _, err := m.svc.Move(m.ctx, id, "Future"); err != nil {
		if m.handleImmutableError(err) {
			return
		}
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		return
	}
	m.setStatus("Moved to Future")
	*cmds = append(*cmds, m.refreshAll())
}

func (m *Model) applyComplete(cmds *[]tea.Cmd, id string) {
	if id == "" {
		return
	}
	if _, err := m.svc.Complete(m.ctx, id); err != nil {
		if m.handleImmutableError(err) {
			return
		}
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		return
	}
	m.setStatus("Completed")
	*cmds = append(*cmds, m.refreshAll())
}

func (m *Model) applyStrikeEntry(cmds *[]tea.Cmd, id string) {
	if id == "" {
		return
	}
	if _, err := m.svc.Strike(m.ctx, id); err != nil {
		if m.handleImmutableError(err) {
			return
		}
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		return
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
			label = entryLabel(parent)
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
		candidates = append(candidates, parentCandidate{ID: e.ID, Label: entryLabel(e)})
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

func entryLabel(e *entry.Entry) string {
	if e == nil {
		return "<unknown>"
	}
	msg := strings.TrimSpace(e.Message)
	if msg != "" {
		return msg
	}
	if e.Collection != "" {
		return e.Collection
	}
	if e.ID != "" {
		return e.ID
	}
	return "<entry>"
}

// View renders two lists and optional input/help overlays
func (m *Model) View() string {
	left := m.colList.View()
	right := m.renderDetailPane()
	gap := lipgloss.NewStyle().Padding(0, 1).Render

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, gap(" "), right)

	var sections []string
	sections = append(sections, body)

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
	if m.mode == modeHelp {
		helpLines := m.helpLines
		if len(helpLines) == 0 {
			helpLines = buildHelpLines()
		}
		panelStyle := lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).Padding(1, 2)
		sections = append(sections, panelStyle.Render(strings.Join(helpLines, "\n")))
	}
	if m.mode == modeReport {
		if overlay := m.renderReportOverlay(); strings.TrimSpace(overlay) != "" {
			sections = append(sections, overlay)
		}
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
		help = ""
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
	}
	m.bottom.SetHelp(help)
}

func (m *Model) isCalendarActive() bool {
	sel := m.colList.SelectedItem()
	if sel == nil {
		return false
	}
	switch sel.(type) {
	case *indexview.CalendarRowItem, *indexview.CalendarHeaderItem:
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
	m.bottom.SetCommandDefinitions(commandDefinitions)
	m.bottom.UpdateCommandInput(m.input.Value(), m.input.View())
	m.setStatus("COMMAND: :q quit · :today Today · :future Future · :report window")
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
	case indexview.CollectionItem:
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
	case *indexview.CalendarRowItem:
		key = v.Month
	case *indexview.CalendarHeaderItem:
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
		ci, ok := it.(indexview.CollectionItem)
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
	ci, ok := sel.(indexview.CollectionItem)
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
		case indexview.CollectionItem:
			if v.Resolved == resolved || (v.Resolved == "" && v.Name == resolved) {
				return i
			}
		case *indexview.CalendarHeaderItem:
			if resolved == v.Month {
				return i
			}
		case *indexview.CalendarRowItem:
			if strings.HasPrefix(resolved, v.Month+"/") {
				if day := indexview.DayFromPath(resolved); day > 0 && indexview.ContainsDay(v.Days, day) {
					return i
				}
			}
		}
	}
	return -1
}

func indexForName(items []list.Item, name string) int {
	for i, it := range items {
		ci, ok := it.(indexview.CollectionItem)
		if !ok {
			continue
		}
		if ci.Name == name {
			return i
		}
	}
	return -1
}

func (m *Model) buildCollectionItems(cols []string, currentResolved string, now time.Time) []list.Item {
	return indexview.BuildItems(m.indexState, cols, currentResolved, now)
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
			desc.name = friendlyCollectionName(desc.id)
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
		ci, ok := it.(indexview.CollectionItem)
		if !ok {
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
			weight := parseDayNumber(lastParent, ci.Name)
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
			children := make([]indexview.CollectionItem, len(st.Children))
			copy(children, st.Children)
			sort.SliceStable(children, func(i, j int) bool {
				ti := parseDay(id, children[i].Name)
				tj := parseDay(id, children[j].Name)
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
				order := childOrder(id, parseDayNumber(id, child.Name))
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
				order := childOrder(parent, parseDayNumber(parent, child))
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
		ensureDesc(focus, friendlyCollectionName(focus), focus)
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
		name:     friendlyCollectionName(id),
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

func parseDay(parent, child string) time.Time {
	monthTime, err := time.Parse("January 2006", parent)
	if err != nil {
		return time.Time{}
	}
	layout := "January 2, 2006"
	if strings.Contains(child, ",") {
		if t, err := time.Parse(layout, child); err == nil {
			return t
		}
	}
	full := fmt.Sprintf("%s %s", parent, strings.TrimSpace(strings.TrimPrefix(child, parent)))
	if t, err := time.Parse(layout, full); err == nil {
		return t
	}
	if day := indexview.DayNumberFromName(monthTime, child); day > 0 {
		return time.Date(monthTime.Year(), monthTime.Month(), day, 0, 0, 0, 0, time.Local)
	}
	return time.Time{}
}

func parseDayNumber(parent, child string) int {
	if t := parseDay(parent, child); !t.IsZero() {
		return t.Day()
	}
	return 0
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

func (m *Model) cancelConfirm() {
	m.confirmAction = confirmNone
	m.confirmTargetID = ""
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

func friendlyCollectionName(id string) string {
	if strings.Contains(id, "/") {
		parts := strings.SplitN(id, "/", 2)
		if len(parts) == 2 {
			if t, err := time.Parse("January 2, 2006", parts[1]); err == nil {
				return t.Format("Monday, January 2, 2006")
			}
			if mt, err := time.Parse("January 2006", parts[0]); err == nil {
				return mt.Format("January, 2006")
			}
		}
	}
	if t, err := time.Parse("January 2, 2006", id); err == nil {
		return t.Format("Monday, January 2, 2006")
	}
	if t, err := time.Parse("January 2006", id); err == nil {
		return t.Format("January, 2006")
	}
	return id
}

func visibilityLabel(show bool) string {
	if show {
		return "visible"
	}
	return "hidden"
}

func (m *Model) pruneHiddenCollections(visible map[string]bool, cmds *[]tea.Cmd) {
	if visible == nil {
		return
	}
	items := m.colList.Items()
	if len(items) == 0 {
		return
	}
	newItems := make([]list.Item, 0, len(items))
	removed := false
	for _, it := range items {
		switch v := it.(type) {
		case indexview.CollectionItem:
			resolved := v.Resolved
			if resolved == "" {
				resolved = v.Name
			}
			isLeaf := v.Indent || !v.HasChildren
			if isLeaf && !visible[resolved] {
				removed = true
				continue
			}
		}
		newItems = append(newItems, it)
	}
	if !removed {
		return
	}
	current := m.selectedCollection()
	m.colList.SetItems(newItems)
	if len(newItems) == 0 {
		return
	}
	idx := indexForResolved(newItems, current)
	if idx < 0 {
		idx = 0
	}
	if idx >= 0 && idx < len(newItems) {
		m.colList.Select(idx)
	}
	m.updateActiveMonthFromSelection(false, cmds)
	if cmd := m.syncCollectionIndicators(); cmd != nil {
		*cmds = append(*cmds, cmd)
	}
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

func formatReportTime(t time.Time) string {
	if t.IsZero() {
		return "(unknown)"
	}
	return t.Local().Format("2006-01-02 15:04")
}

func (m *Model) reportVisibleHeight() int {
	height := m.termHeight - m.bottom.Height() - 4
	if height < 3 {
		height = 3
	}
	return height
}

func (m *Model) ensureReportBounds() {
	if len(m.reportLines) == 0 {
		m.reportOffset = 0
		return
	}
	height := m.reportVisibleHeight()
	maxOffset := len(m.reportLines) - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.reportOffset > maxOffset {
		m.reportOffset = maxOffset
	}
	if m.reportOffset < 0 {
		m.reportOffset = 0
	}
}

func (m *Model) renderReportOverlay() string {
	if len(m.reportLines) == 0 {
		return ""
	}
	m.ensureReportBounds()
	height := m.reportVisibleHeight()
	if height > len(m.reportLines) {
		height = len(m.reportLines)
	}
	end := m.reportOffset + height
	if end > len(m.reportLines) {
		end = len(m.reportLines)
	}
	viewport := m.reportLines[m.reportOffset:end]
	width := m.termWidth - 6
	if width < 20 {
		width = 20
	}
	padded := make([]string, len(viewport))
	for i, line := range viewport {
		padded[i] = padRight(line, width)
	}
	boxStyle := lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).Padding(1, 2)
	return boxStyle.Width(width + 4).Render(strings.Join(padded, "\n"))
}

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func (m *Model) handleReportKey(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "esc", "q":
		m.exitReportMode()
		return true
	case "j", "down":
		m.reportOffset++
	case "k", "up":
		m.reportOffset--
	case "pgdown", "space", "ctrl+f":
		m.reportOffset += m.reportVisibleHeight()
	case "pgup", "ctrl+b":
		m.reportOffset -= m.reportVisibleHeight()
	case "g", "home":
		m.reportOffset = 0
	case "G", "end":
		m.reportOffset = len(m.reportLines)
	default:
		return false
	}
	m.ensureReportBounds()
	return true
}

func (m *Model) exitReportMode() {
	m.reportSections = nil
	m.reportLines = nil
	m.reportOffset = 0
	m.reportLabel = ""
	m.reportTotal = 0
	m.setMode(modeNormal)
	m.setOverlayReserve(0)
	m.setStatus("Report closed")
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
	_, ok := it.(*indexview.CalendarHeaderItem)
	return ok
}

func (m *Model) markCalendarSelection() tea.Cmd {
	sel := m.colList.SelectedItem()
	switch v := sel.(type) {
	case *indexview.CalendarHeaderItem:
		state := m.indexState.Months[v.Month]
		if state == nil || len(state.Weeks) == 0 {
			return nil
		}
		m.colList.Select(state.Weeks[0].RowIndex)
		return m.markCalendarSelection()
	case *indexview.CalendarRowItem:
		state := m.indexState.Months[v.Month]
		if state == nil {
			return nil
		}
		day := m.indexState.Selection[v.Month]
		if day == 0 || !indexview.ContainsDay(v.Days, day) {
			day = indexview.FirstNonZero(v.Days)
		}
		if day == 0 {
			return nil
		}
		m.indexState.Selection[v.Month] = day
		m.pendingResolved = indexview.FormatDayPath(state.MonthTime, day)
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
	case *indexview.CalendarRowItem:
		month = v.Month
	case *indexview.CalendarHeaderItem:
		month = v.Month
	default:
		return nil
	}

	state := m.indexState.Months[month]
	if state == nil || len(state.Weeks) == 0 {
		return nil
	}

	selected := m.indexState.Selection[month]
	if selected == 0 {
		selected = indexview.DefaultSelectedDay(month, state.MonthTime, state.Children, m.pendingResolved, time.Now())
		if selected == 0 {
			selected = indexview.FirstNonZero(state.Weeks[0].Days)
		}
	}
	if selected == 0 {
		return nil
	}

	newDay := selected + dx + dy*7
	daysInMonth := indexview.DaysIn(state.MonthTime)
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
	m.pendingResolved = indexview.FormatDayPath(state.MonthTime, newDay)
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

func (m *Model) findWeekForDay(month string, day int) *indexview.CalendarRowItem {
	state := m.indexState.Months[month]
	if state == nil {
		return nil
	}
	for _, week := range state.Weeks {
		if indexview.ContainsDay(week.Days, day) {
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
	if _, ok := items[idx].(*indexview.CalendarRowItem); ok {
		if day := indexview.DayFromPath(resolved); day > 0 {
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
	day := indexview.DayFromPath(resolved)
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

	header, weeks := indexview.RenderCalendarRows(month, state.MonthTime, state.Children, selected, time.Now(), indexview.DefaultCalendarOptions())
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

	oldCount := len(state.Weeks)
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

	state.Weeks = weeks
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
	case *indexview.CalendarRowItem:
		m.applyActiveCalendarMonth(v.Month, force, cmds)
	case *indexview.CalendarHeaderItem:
		m.applyActiveCalendarMonth(v.Month, force, cmds)
	default:
		m.applyActiveCalendarMonth("", false, cmds)
	}
}
