package teaui

import (
	"context"
	"fmt"
	"sort"
	"strings"
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
)

// Model states and actions
type mode int

const (
	modeNormal mode = iota
	modeInsert
	modeCommand
	modeHelp
	modeBulletSelect
)

type action int

const (
	actionNone action = iota
	actionAdd
	actionEdit
	actionMove
)

type collectionDescriptor struct {
	id       string
	name     string
	resolved string
}

var commandDefinitions = []bottombar.CommandOption{
	{Name: "q", Description: "Quit application"},
	{Name: "quit", Description: "Quit application"},
	{Name: "exit", Description: "Quit application"},
	{Name: "today", Description: "Jump to Today collection"},
}

// Model contains UI state
type Model struct {
	svc        *app.Service
	ctx        context.Context
	mode       mode
	resumeMode mode
	action     action

	focus int // 0: collections, 1: entries

	colList list.Model

	input textinput.Model

	pendingBullet  glyph.Bullet
	bulletOptions  []glyph.Bullet
	bulletIndex    int
	bulletTargetID string
	awaitingDD     bool
	lastDTime      time.Time

	termWidth       int
	termHeight      int
	verticalReserve int
	overlayReserve  int
	indexState      *indexview.State
	pendingResolved string
	detailWidth     int
	detailHeight    int
	detailState     *detailview.State
	entriesCache    map[string][]*entry.Entry
	detailOrder     []collectionDescriptor

	focusDel list.DefaultDelegate
	blurDel  list.DefaultDelegate

	bottom bottombar.Model
}

// New creates a new UI model backed by the Service.
func New(svc *app.Service) Model {
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
	l1.Title = "Index"
	l1.SetShowHelp(false)
	l1.SetShowStatusBar(false)

	ti := textinput.New()
	ti.Placeholder = "Type here"
	ti.CharLimit = 256
	ti.Focus()
	ti.Prompt = ""
	ti.Styles.Cursor.Color = lipgloss.Color("218")
	ti.Styles.Cursor.Shape = tea.CursorUnderline

	bulletOpts := []glyph.Bullet{glyph.Task, glyph.Note, glyph.Event, glyph.Completed, glyph.Irrelevant}

	bottom := bottombar.New()

	m := Model{
		svc:           svc,
		ctx:           context.Background(),
		mode:          modeNormal,
		action:        actionNone,
		focus:         1,
		colList:       l1,
		input:         ti,
		pendingBullet: glyph.Task,
		focusDel:      dFocus,
		blurDel:       dBlur,
		bulletOptions: bulletOpts,
		indexState:    indexview.NewState(),
		bottom:        bottom,
		resumeMode:    modeNormal,
		detailState:   detailview.NewState(),
		entriesCache:  make(map[string][]*entry.Entry),
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
func (m Model) Init() tea.Cmd {
	return m.refreshAll()
}

func (m *Model) refreshAll() tea.Cmd {
	m.entriesCache = make(map[string][]*entry.Entry)
	return m.loadCollections()
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
		sections := make([]detailview.Section, 0, len(order))

		addSection := func(desc collectionDescriptor, entries []*entry.Entry) {
			name := desc.name
			if name == "" {
				name = friendlyCollectionName(desc.id)
			}
			sections = append(sections, detailview.Section{
				CollectionID:   desc.id,
				CollectionName: name,
				ResolvedName:   desc.resolved,
				Entries:        entries,
			})
			seen[desc.id] = true
		}

		for _, desc := range order {
			if desc.id == "" || seen[desc.id] {
				continue
			}
			entries, err := m.entriesForCollection(desc.id)
			if err != nil {
				return errMsg{err}
			}
			if len(entries) == 0 && desc.id != focus {
				continue
			}
			addSection(desc, entries)
		}

		if focus != "" && !seen[focus] {
			entries, err := m.entriesForCollection(focus)
			if err != nil {
				return errMsg{err}
			}
			addSection(m.descriptorForCollection(focus), entries)
		}

		if len(sections) == 0 && focus != "" {
			entries, err := m.entriesForCollection(focus)
			if err != nil {
				return errMsg{err}
			}
			addSection(m.descriptorForCollection(focus), entries)
		}

		if len(sections) == 0 {
			return detailSectionsLoadedMsg{sections: sections, activeCollection: focus, activeEntry: ""}
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

		return detailSectionsLoadedMsg{sections: sections, activeCollection: focus, activeEntry: focusEntry}
	}
}

// messages
type errMsg struct{ err error }
type collectionsLoadedMsg struct{ items []list.Item }
type detailSectionsLoadedMsg struct {
	sections         []detailview.Section
	activeCollection string
	activeEntry      string
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
	case modeNormal:
		return m.handleNormalKey(msg, cmds)
	default:
		return false
	}
}

func (m *Model) handleHelpKey(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "q", "esc", "?":
		m.setMode(modeNormal)
		m.setOverlayReserve(0)
		return true
	default:
		return false
	}
}

func (m *Model) handleBulletSelectKey(msg tea.KeyPressMsg, cmds *[]tea.Cmd) bool {
	switch msg.String() {
	case "esc", "q":
		m.exitBulletSelect(cmds)
		return true
	case "enter":
		chosen := m.bulletOptions[m.bulletIndex]
		if m.bulletTargetID == "" {
			m.pendingBullet = chosen
			m.bottom.SetPendingBullet(m.pendingBullet)
			m.setStatus(fmt.Sprintf("Default bullet set to %s", chosen.Glyph().Meaning))
		} else {
			m.applySetBullet(cmds, m.bulletTargetID, chosen)
		}
		m.exitBulletSelect(cmds)
		return true
	case "up", "k":
		if m.bulletIndex > 0 {
			m.bulletIndex--
		} else {
			m.bulletIndex = len(m.bulletOptions) - 1
		}
	case "down", "j":
		if m.bulletIndex < len(m.bulletOptions)-1 {
			m.bulletIndex++
		} else {
			m.bulletIndex = 0
		}
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
		m.enterBulletSelect("", m.pendingBullet)
		return true
	case "ctrl+t":
		m.pendingBullet = glyph.Task
		m.bottom.SetPendingBullet(m.pendingBullet)
		m.setStatus("Compose bullet set to Task")
		return true
	case "ctrl+n":
		m.pendingBullet = glyph.Note
		m.bottom.SetPendingBullet(m.pendingBullet)
		m.setStatus("Compose bullet set to Note")
		return true
	case "ctrl+e":
		m.pendingBullet = glyph.Event
		m.bottom.SetPendingBullet(m.pendingBullet)
		m.setStatus("Compose bullet set to Event")
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
		m.setMode(modeNormal)
		m.input.Reset()
		m.input.Blur()
		m.bottom.UpdateCommandInput("", "")
		m.setStatus("Command cancelled")
		m.setOverlayReserve(0)
		return true
	case "up":
		if opt, ok := m.bottom.Suggestion(0); ok {
			m.input.SetValue(opt.Name)
			m.input.CursorEnd()
			m.bottom.UpdateCommandInput(m.input.Value(), m.input.View())
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
			*cmds = append(*cmds, m.loadDetailSections())
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
		*cmds = append(*cmds, m.loadDetailSections())
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
				*cmds = append(*cmds, m.loadDetailSectionsWithFocus(target, ""))
			}
			return true
		}
	case "j", "down":
		if m.focus == 0 {
			if cmd := m.moveCalendarCursor(0, 1); cmd != nil {
				*cmds = append(*cmds, cmd)
				return true
			}
			m.colList.CursorDown()
			m.ensureCollectionSelection(1)
			m.updateActiveMonthFromSelection(false, cmds)
			*cmds = append(*cmds, m.loadDetailSections())
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
			*cmds = append(*cmds, m.loadDetailSections())
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
			*cmds = append(*cmds, m.markCalendarSelection())
		} else {
			m.detailState.ScrollToTop()
			sections := m.detailState.Sections()
			if len(sections) > 0 {
				m.selectCollectionByID(sections[0].CollectionID, cmds)
				*cmds = append(*cmds, m.loadDetailSections())
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
			*cmds = append(*cmds, m.markCalendarSelection())
		} else {
			sections := m.detailState.Sections()
			if len(sections) > 0 {
				last := sections[len(sections)-1].CollectionID
				m.detailState.SetActive(last, "")
				m.selectCollectionByID(last, cmds)
				*cmds = append(*cmds, m.loadDetailSections())
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
	case "o", "O":
		m.setMode(modeInsert)
		m.action = actionAdd
		m.input.Placeholder = "New item message"
		m.input.SetValue("")
		if cmd := m.input.Focus(); cmd != nil {
			*cmds = append(*cmds, cmd)
		}
		*cmds = append(*cmds, textinput.Blink)
		return true
	case "i":
		if it := m.currentEntry(); it != nil {
			m.setMode(modeInsert)
			m.action = actionEdit
			m.input.Placeholder = "Edit message"
			m.input.SetValue(it.Message)
			m.input.CursorEnd()
			if cmd := m.input.Focus(); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
			*cmds = append(*cmds, textinput.Blink)
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
	case "t":
		m.pendingBullet = glyph.Task
		m.bottom.SetPendingBullet(m.pendingBullet)
	case "n":
		m.pendingBullet = glyph.Note
		m.bottom.SetPendingBullet(m.pendingBullet)
	case "e":
		m.pendingBullet = glyph.Event
		m.bottom.SetPendingBullet(m.pendingBullet)
	case "b":
		var target string
		current := m.pendingBullet
		if m.focus == 1 {
			if it := m.currentEntry(); it != nil {
				target = it.ID
				current = it.Bullet
			}
		}
		m.enterBulletSelect(target, current)
		return true
	case "T":
		if it := m.currentEntry(); it != nil {
			m.applySetBullet(cmds, it.ID, glyph.Task)
		}
		return true
	case "N":
		if it := m.currentEntry(); it != nil {
			m.applySetBullet(cmds, it.ID, glyph.Note)
		}
		return true
	case "E":
		if it := m.currentEntry(); it != nil {
			m.applySetBullet(cmds, it.ID, glyph.Event)
		}
		return true
	case "*":
		if it := m.currentEntry(); it != nil {
			m.applyToggleSig(cmds, it.ID, glyph.Priority)
		}
		return true
	case "!":
		if it := m.currentEntry(); it != nil {
			m.applyToggleSig(cmds, it.ID, glyph.Inspiration)
		}
		return true
	case "?":
		if m.focus == 1 {
			if it := m.currentEntry(); it != nil {
				m.applyToggleSig(cmds, it.ID, glyph.Investigation)
			}
			return true
		}
		m.setMode(modeHelp)
		m.setOverlayReserve(3)
		return true
	case "f1":
		m.setMode(modeHelp)
		m.setOverlayReserve(3)
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
		m.applyAdd(cmds, m.selectedCollection(), input)
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
	switch input {
	case "q", "quit", "exit":
		*cmds = append(*cmds, tea.Quit)
	case "today":
		if cmd := m.selectToday(); cmd != nil {
			*cmds = append(*cmds, cmd)
		}
	case "":
		// no-op
	default:
		m.setStatus(fmt.Sprintf("Unknown command: %s", input))
	}
	m.setMode(modeNormal)
	m.input.Reset()
	m.input.Blur()
	m.bottom.UpdateCommandInput("", "")
	m.setOverlayReserve(0)
}

// Update handles messages and keybindings
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		m.detailState.SetActive(msg.activeCollection, msg.activeEntry)
		m.updateBottomContext()
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
	if collection == "" || message == "" {
		return
	}
	if _, err := m.svc.Add(m.ctx, collection, m.pendingBullet, message, glyph.None); err != nil {
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		return
	}
	m.setStatus("Added")
	*cmds = append(*cmds, m.refreshAll())
}

func (m *Model) applyEdit(cmds *[]tea.Cmd, id, message string) {
	if id == "" || message == "" {
		return
	}
	if _, err := m.svc.Edit(m.ctx, id, message); err != nil {
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
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
		return
	}
	m.setStatus("Struck")
	*cmds = append(*cmds, m.refreshAll())
}

func (m *Model) applySetBullet(cmds *[]tea.Cmd, id string, b glyph.Bullet) {
	if _, err := m.svc.SetBullet(m.ctx, id, b); err != nil {
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
	} else {
		m.setStatus("Bullet updated")
		*cmds = append(*cmds, m.refreshAll())
	}
}

func (m *Model) applyToggleSig(cmds *[]tea.Cmd, id string, s glyph.Signifier) {
	if _, err := m.svc.ToggleSignifier(m.ctx, id, s); err != nil {
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
	} else {
		m.setStatus("Signifier toggled")
		*cmds = append(*cmds, m.refreshAll())
	}
}

// View renders two lists and optional input/help overlays
func (m Model) View() string {
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
		lines := []string{"Select bullet (enter to confirm, esc to cancel):"}
		for i, b := range m.bulletOptions {
			glyphInfo := b.Glyph()
			indicator := "  "
			if i == m.bulletIndex {
				indicator = "→ "
			}
			lines = append(lines, fmt.Sprintf("%s%s %s", indicator, glyphInfo.Symbol, glyphInfo.Meaning))
		}
		panelStyle := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(1, 2)
		sections = append(sections, panelStyle.Render(strings.Join(lines, "\n")))
	}
	if m.mode == modeHelp {
		help := "Keys: ←/→ switch panes, ↑/↓ move, gg/G top/bottom, [/] fold, o add, i edit, x complete, dd strike, > move, < future, t/n/e set add-bullet, T/N/E set on item, */!/?: toggle signifiers, :q quit, :today jump"
		sections = append(sections, lipgloss.NewStyle().Italic(true).Render(help))
	}

	if footer, _ := m.bottom.View(); footer != "" {
		sections = append(sections, footer)
	}

	return strings.Join(sections, "\n\n")
}

func (m *Model) renderDetailPane() string {
	header := "<empty>"
	var sections []detailview.Section
	if m.detailState != nil {
		sections = m.detailState.Sections()
	}
	if len(sections) > 0 {
		name := sections[0].CollectionName
		if name == "" {
			if sections[0].ResolvedName != "" {
				name = sections[0].ResolvedName
			} else {
				name = sections[0].CollectionID
			}
		}
		header = friendlyCollectionName(name)
	}
	headerStyle := lipgloss.NewStyle().Bold(true)
	if m.focus == 1 {
		headerStyle = headerStyle.Foreground(lipgloss.Color("213"))
	} else {
		headerStyle = headerStyle.Foreground(lipgloss.Color("244"))
	}
	contentHeight := m.detailHeight
	if contentHeight <= 0 {
		contentHeight = len(sections) * 4
		if contentHeight < 1 {
			contentHeight = 1
		}
	}
	content := ""
	if m.detailState != nil {
		content, _ = m.detailState.Viewport(contentHeight)
	}
	if strings.TrimSpace(content) == "" {
		return headerStyle.Render(header)
	}
	return headerStyle.Render(header) + "\n" + content
}

// Program entry
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
			help = "Compose · Enter save · Esc cancel · ctrl+b bullet menu · ctrl+t/n/e set bullet"
		case actionEdit:
			help = "Edit · Enter save · Esc cancel"
		case actionMove:
			help = "Move · Enter confirm · Esc cancel"
		default:
			help = "Compose · Enter save · Esc cancel"
		}
	case modeHelp:
		help = "Help · q close"
	case modeBulletSelect:
		help = "Select bullet · Enter confirm · Esc cancel · j/k cycle"
	default:
		if m.focus == 0 {
			if m.isCalendarActive() {
				help = "Index · h/l day · j/k week · enter focus · o add entry · { fold · } expand month · F1 help"
			} else {
				help = "Index · h/l panes · j/k move · o add entry · { collapse · } expand · : command mode · F1 help"
			}
		} else {
			help = "Entries · j/k move · PgUp/PgDn or cmd+↑/↓ switch collection · i edit · x complete · dd strike · b bullet menu · > move · * priority · ! inspiration · ? investigate"
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
	m.colList.Title = "Collections"
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

func (m *Model) enterBulletSelect(targetID string, current glyph.Bullet) {
	prevMode := m.mode
	m.setMode(modeBulletSelect)
	m.resumeMode = prevMode
	if prevMode == modeInsert {
		m.input.Blur()
	}
	m.bulletTargetID = targetID
	m.bulletIndex = m.findBulletIndex(current)
	reserve := len(m.bulletOptions) + 5
	m.setOverlayReserve(reserve)
	if targetID == "" {
		m.setStatus("Choose default bullet for new entries")
	} else {
		m.setStatus("Choose bullet for selected entry")
	}
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
	m.setOverlayReserve(0)
}

func (m *Model) enterCommandMode(cmds *[]tea.Cmd) {
	m.setMode(modeCommand)
	m.input.Reset()
	m.input.Placeholder = "command"
	m.input.CursorEnd()
	if cmd := m.input.Focus(); cmd != nil {
		*cmds = append(*cmds, cmd)
	}
	*cmds = append(*cmds, textinput.Blink)
	m.bottom.SetCommandDefinitions(commandDefinitions)
	m.bottom.UpdateCommandInput(m.input.Value(), m.input.View())
	m.setStatus("COMMAND: :q to quit, :today jump to Today")
	m.applyReserve()
}

func (m *Model) selectToday() tea.Cmd {
	monthLabel, dayLabel, resolved := todayLabels()
	if t, err := time.Parse("January 2, 2006", dayLabel); err == nil {
		m.indexState.Selection[monthLabel] = t.Day()
	}
	m.indexState.Fold[monthLabel] = false
	m.pendingResolved = resolved
	m.focus = 1
	m.updateFocusHeaders()
	m.updateBottomContext()
	m.setOverlayReserve(0)
	m.setStatus(fmt.Sprintf("Selected Today (%s)", dayLabel))

	cmds := []tea.Cmd{m.loadCollections()}
	cmds = append(cmds, m.loadDetailSectionsWithFocus(resolved, ""))
	if cmd := m.syncCollectionIndicators(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func (m *Model) toggleFoldCurrent(explicit *bool) tea.Cmd {
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
	seen := make(map[string]bool)
	var descriptors []collectionDescriptor

	appendDesc := func(id, name, resolved string) {
		if id == "" || seen[id] {
			return
		}
		if name == "" {
			name = friendlyCollectionName(id)
		}
		descriptors = append(descriptors, collectionDescriptor{id: id, name: name, resolved: resolved})
		seen[id] = true
	}

	for _, it := range m.colList.Items() {
		switch v := it.(type) {
		case indexview.CollectionItem:
			id := v.Resolved
			if id == "" {
				id = v.Name
			}
			if _, ok := indexview.ParseMonth(v.Name); ok {
				state := m.indexState.Months[id]
				appendDesc(id, v.Name, v.Resolved)
				if state != nil {
					for _, child := range state.Children {
						childID := child.Resolved
						if childID == "" {
							childID = fmt.Sprintf("%s/%s", id, child.Name)
						}
						name := child.Name
						if name == "" {
							name = friendlyCollectionName(childID)
						}
						appendDesc(childID, name, child.Resolved)
					}
				}
				continue
			}
			if v.Indent {
				// month decay handled above
				continue
			}
			appendDesc(id, v.Name, v.Resolved)
		}
	}

	focus := m.selectedCollection()
	if m.detailState != nil {
		if active := m.detailState.ActiveCollectionID(); active != "" {
			focus = active
		}
		for _, sec := range m.detailState.Sections() {
			appendDesc(sec.CollectionID, sec.CollectionName, sec.ResolvedName)
		}
	}
	if focus != "" {
		appendDesc(focus, friendlyCollectionName(focus), focus)
	}

	return descriptors
}

func indexOfDescriptor(list []collectionDescriptor, id string) int {
	for i, desc := range list {
		if desc.id == id {
			return i
		}
	}
	return -1
}

func (m *Model) descriptorForCollection(id string) collectionDescriptor {
	return collectionDescriptor{
		id:       id,
		name:     friendlyCollectionName(id),
		resolved: id,
	}
}

func (m *Model) entriesForCollection(collection string) ([]*entry.Entry, error) {
	if collection == "" {
		return nil, nil
	}
	if entries, ok := m.entriesCache[collection]; ok {
		return entries, nil
	}
	entries, err := m.svc.Entries(m.ctx, collection)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Created.Time.Before(entries[j].Created.Time)
	})
	m.entriesCache[collection] = entries
	return entries, nil
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

func monthComponent(id string) string {
	if strings.Contains(id, "/") {
		parts := strings.SplitN(id, "/", 2)
		return parts[0]
	}
	if _, err := time.Parse("January 2006", id); err == nil {
		return id
	}
	return ""
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

func (m *Model) ensureCalendarHighlight() {}

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

	var cmds []tea.Cmd
	m.applyActiveCalendarMonth(month, true, &cmds)
	if week := m.findWeekForDay(month, newDay); week != nil {
		if m.colList.Index() != week.RowIndex {
			m.colList.Select(week.RowIndex)
		}
	}
	cmds = append(cmds, m.loadDetailSections())
	return tea.Batch(cmds...)
}

func (m *Model) moveDetailCursor(delta int, cmds *[]tea.Cmd) bool {
	if m.detailState == nil {
		return false
	}
	prevCollection := m.detailState.ActiveCollectionID()
	m.detailState.MoveEntry(delta)
	currentEntry := m.detailState.ActiveEntryID()
	newCollection := m.detailState.ActiveCollectionID()
	if newCollection != "" && newCollection != prevCollection {
		m.pendingResolved = newCollection
		if !m.selectCollectionByID(newCollection, cmds) {
			*cmds = append(*cmds, m.loadDetailSectionsWithFocus(newCollection, currentEntry))
			m.updateBottomContext()
			return true
		}
		*cmds = append(*cmds, m.loadDetailSectionsWithFocus(newCollection, currentEntry))
		return true
	}
	m.updateBottomContext()
	return true
}

func (m *Model) moveDetailSection(delta int, cmds *[]tea.Cmd) bool {
	if m.detailState == nil {
		return false
	}
	prevCollection := m.detailState.ActiveCollectionID()
	m.detailState.MoveSection(delta)
	newCollection := m.detailState.ActiveCollectionID()
	if newCollection == "" {
		return false
	}
	if newCollection != prevCollection {
		m.pendingResolved = newCollection
		if !m.selectCollectionByID(newCollection, cmds) {
			*cmds = append(*cmds, m.loadDetailSectionsWithFocus(newCollection, ""))
			m.updateBottomContext()
			return true
		}
		*cmds = append(*cmds, m.loadDetailSectionsWithFocus(newCollection, ""))
		return true
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
func daysIn(month time.Time) int {
	first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
	return first.AddDate(0, 1, -1).Day()
}
