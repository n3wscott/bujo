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

const todayMetaName = "Today"

var commandDefinitions = []bottombar.CommandOption{
	{Name: "q", Description: "Quit application"},
	{Name: "quit", Description: "Quit application"},
	{Name: "exit", Description: "Quit application"},
	{Name: "today", Description: "Jump to Today collection"},
}

// entry item for right list
type entryItem struct{ e *entry.Entry }

func (it entryItem) Title() string {
	return it.e.String()
}
func (it entryItem) Description() string { return "" }
func (it entryItem) FilterValue() string { return it.e.Message }

// Model contains UI state
type Model struct {
	svc        *app.Service
	ctx        context.Context
	mode       mode
	resumeMode mode
	action     action

	focus int // 0: collections, 1: entries

	colList list.Model
	entList list.Model

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

	l2 := list.New([]list.Item{}, dFocus, 80, 20)
	l2.Title = "<empty>"
	l2.SetShowHelp(false)
	l2.SetShowStatusBar(false)

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
		entList:       l2,
		input:         ti,
		pendingBullet: glyph.Task,
		focusDel:      dFocus,
		blurDel:       dBlur,
		bulletOptions: bulletOpts,
		indexState:    indexview.NewState(),
		bottom:        bottom,
		resumeMode:    modeNormal,
	}
	m.bulletIndex = m.findBulletIndex(m.pendingBullet)
	m.bottom.SetPendingBullet(m.pendingBullet)
	m.bottom.SetMode(bottombar.ModeNormal)
	m.updateBottomContext()
	m.applyReserve()
	m.updateFocusHeaders()
	return m
}

// Init loads initial data
func (m Model) Init() tea.Cmd {
	return m.refreshAll()
}

func (m *Model) refreshAll() tea.Cmd {
	return tea.Batch(m.loadCollections(), m.loadEntries())
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

func (m *Model) loadEntries() tea.Cmd {
	col := m.selectedCollection()
	return func() tea.Msg {
		if col == "" {
			return entriesLoadedMsg{nil}
		}
		ents, err := m.svc.Entries(m.ctx, col)
		if err != nil {
			return errMsg{err}
		}
		sort.SliceStable(ents, func(i, j int) bool {
			return ents[i].Created.Time.Before(ents[j].Created.Time)
		})
		items := make([]list.Item, 0, len(ents))
		for _, e := range ents {
			items = append(items, entryItem{e: e})
		}
		return entriesLoadedMsg{items}
	}
}

// messages
type errMsg struct{ err error }
type collectionsLoadedMsg struct{ items []list.Item }
type entriesLoadedMsg struct{ items []list.Item }

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
			*cmds = append(*cmds, m.loadEntries())
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
		*cmds = append(*cmds, m.loadEntries())
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
		*cmds = append(*cmds, m.loadEntries())
		if cmd := m.syncCollectionIndicators(); cmd != nil {
			*cmds = append(*cmds, cmd)
		}
		return true
	case "enter":
		if m.focus == 0 {
			if cmd := m.markCalendarSelection(); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
			if m.isCalendarActive() {
				m.focus = 1
				m.updateFocusHeaders()
				m.updateBottomContext()
				if cmd := m.syncCollectionIndicators(); cmd != nil {
					*cmds = append(*cmds, cmd)
				}
			}
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
			*cmds = append(*cmds, m.loadEntries())
			return true
		}
		m.entList.CursorDown()
		return true
	case "k", "up":
		if m.focus == 0 {
			if cmd := m.moveCalendarCursor(0, -1); cmd != nil {
				*cmds = append(*cmds, cmd)
				return true
			}
			m.colList.CursorUp()
			m.ensureCollectionSelection(-1)
			m.updateActiveMonthFromSelection(false, cmds)
			*cmds = append(*cmds, m.loadEntries())
			return true
		}
		m.entList.CursorUp()
		return true
	case "g":
		if m.focus == 0 {
			m.colList.Select(0)
			if cmd := m.syncCollectionIndicators(); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
			m.updateActiveMonthFromSelection(false, cmds)
			*cmds = append(*cmds, m.markCalendarSelection())
		} else {
			m.entList.Select(0)
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
			m.entList.Select(len(m.entList.Items()) - 1)
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
			m.input.SetValue(it.e.Message)
			m.input.CursorEnd()
			if cmd := m.input.Focus(); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
			*cmds = append(*cmds, textinput.Blink)
			return true
		}
	case "x":
		if it := m.currentEntry(); it != nil {
			m.applyComplete(cmds, it.e.ID)
		}
		return true
	case "d":
		if it := m.currentEntry(); it != nil {
			if m.awaitingDD && time.Since(m.lastDTime) < 600*time.Millisecond {
				m.applyStrikeEntry(cmds, it.e.ID)
				m.awaitingDD = false
			} else {
				m.awaitingDD = true
				m.lastDTime = time.Now()
			}
		}
		return true
	case ">":
		if m.currentEntry() != nil {
			m.setMode(modeInsert)
			m.action = actionMove
			m.input.Placeholder = "Move to collection"
			m.input.SetValue("")
			if cmd := m.input.Focus(); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
			*cmds = append(*cmds, textinput.Blink)
			return true
		}
	case "<":
		if it := m.currentEntry(); it != nil {
			m.applyMoveToFuture(cmds, it.e.ID)
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
				target = it.e.ID
				current = it.e.Bullet
			}
		}
		m.enterBulletSelect(target, current)
		return true
	case "T":
		if it := m.currentEntry(); it != nil {
			m.applySetBullet(cmds, it.e.ID, glyph.Task)
		}
		return true
	case "N":
		if it := m.currentEntry(); it != nil {
			m.applySetBullet(cmds, it.e.ID, glyph.Note)
		}
		return true
	case "E":
		if it := m.currentEntry(); it != nil {
			m.applySetBullet(cmds, it.e.ID, glyph.Event)
		}
		return true
	case "*":
		if it := m.currentEntry(); it != nil {
			m.applyToggleSig(cmds, it.e.ID, glyph.Priority)
		}
		return true
	case "!":
		if it := m.currentEntry(); it != nil {
			m.applyToggleSig(cmds, it.e.ID, glyph.Inspiration)
		}
		return true
	case "?":
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
			m.applyEdit(cmds, it.e.ID, input)
		}
	case actionMove:
		if it := m.currentEntry(); it != nil {
			m.applyMove(cmds, it.e.ID, input)
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
				targetIdx = indexForName(msg.items, todayMetaName)
			}
			if targetIdx == -1 {
				targetIdx = 0
			}
			m.colList.Select(targetIdx)
			m.updateActiveMonthFromSelection(false, &cmds)
			if _, ok := m.colList.SelectedItem().(*indexview.CalendarRowItem); ok {
				cmds = append(cmds, m.markCalendarSelection())
			}
		}
		if cmd := m.syncCollectionIndicators(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.markCalendarSelection())
		cmds = append(cmds, m.loadEntries())
		cmds = append(cmds, m.markCalendarSelection())
		m.updateBottomContext()
	case entriesLoadedMsg:
		m.entList.SetItems(msg.items)
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
				cmds = append(cmds, m.loadEntries())
				m.updateBottomContext()
			}
			if cmd := m.syncCollectionIndicators(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		} else {
			var cmd tea.Cmd
			m.entList, cmd = m.entList.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) currentEntry() *entryItem {
	if len(m.entList.Items()) == 0 {
		return nil
	}
	sel := m.entList.SelectedItem()
	if sel == nil {
		return nil
	}
	it, _ := sel.(entryItem)
	return &it
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
	m.updateEntriesTitle()
	right := m.entList.View()
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
	m.entList.SetSize(right, height)
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
				help = "Index · h/l day · j/k week · enter focus · o add entry · { fold · } expand month"
			} else {
				help = "Index · h/l panes · j/k move · o add entry · { collapse · } expand · : command mode"
			}
		} else {
			help = "Entries · j/k move · i edit · x complete · dd strike · b bullet menu · > move"
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
	m.entList.Title = "Entries"
	if m.focus == 0 {
		m.colList.SetDelegate(m.focusDel)
		m.entList.SetDelegate(m.blurDel)
	} else {
		m.colList.SetDelegate(m.blurDel)
		m.entList.SetDelegate(m.focusDel)
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
	month, todayDay, todayResolved := todayLabels()
	m.indexState.Fold[month] = false
	items := m.colList.Items()
	var updateCmds []tea.Cmd
	targetIdx := -1
	for i, it := range items {
		ci, ok := it.(indexview.CollectionItem)
		if !ok {
			continue
		}
		if ci.Name == todayMetaName {
			targetIdx = i
			if ci.Resolved != todayResolved {
				ci.Resolved = todayResolved
				if cmd := m.colList.SetItem(i, ci); cmd != nil {
					updateCmds = append(updateCmds, cmd)
				}
			}
			break
		}
	}
	if targetIdx == -1 {
		ci := indexview.CollectionItem{Name: todayMetaName, Resolved: todayResolved}
		if cmd := m.colList.InsertItem(0, ci); cmd != nil {
			updateCmds = append(updateCmds, cmd)
		}
		targetIdx = 0
	}
	m.colList.Select(targetIdx)
	var activeCmds []tea.Cmd
	m.updateActiveMonthFromSelection(false, &activeCmds)
	m.focus = 1
	m.updateFocusHeaders()
	m.updateBottomContext()
	m.setOverlayReserve(0)
	m.setStatus(fmt.Sprintf("Selected Today (%s)", todayDay))

	cmdIndicators := m.syncCollectionIndicators()
	loadEntriesCmd := m.loadEntries()

	allCmds := append([]tea.Cmd{}, updateCmds...)
	allCmds = append(allCmds, activeCmds...)
	if cmdIndicators != nil {
		allCmds = append(allCmds, cmdIndicators)
	}
	if loadEntriesCmd != nil {
		allCmds = append(allCmds, loadEntriesCmd)
	}
	if len(allCmds) == 0 {
		return nil
	}
	return tea.Batch(allCmds...)
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
	return indexview.BuildItems(m.indexState, todayMetaName, cols, currentResolved, now)
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
		cmds = append(cmds, m.loadEntries())
		return tea.Batch(cmds...)
	default:
		return nil
	}
}

func (m *Model) ensureCalendarHighlight() {}

func (m *Model) updateEntriesTitle() {
	col := m.selectedCollection()
	if col == "" {
		m.entList.Title = "<empty>"
		return
	}

	if strings.Contains(col, "/") {
		parts := strings.SplitN(col, "/", 2)
		if len(parts) == 2 {
			month := parts[0]
			day := parts[1]
			if t, err := time.Parse("January 2, 2006", day); err == nil {
				m.entList.Title = t.Format("Monday, January 2, 2006")
				return
			}
			if mt, err := time.Parse("January 2006", month); err == nil {
				m.entList.Title = mt.Format("January, 2006")
				return
			}
		}
	}

	if t, err := time.Parse("January 2006", col); err == nil {
		m.entList.Title = t.Format("January, 2006")
		return
	}

	m.entList.Title = col
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

	var cmds []tea.Cmd
	m.applyActiveCalendarMonth(month, true, &cmds)
	if week := m.findWeekForDay(month, newDay); week != nil {
		if m.colList.Index() != week.RowIndex {
			m.colList.Select(week.RowIndex)
		}
	}
	cmds = append(cmds, m.loadEntries())
	return tea.Batch(cmds...)
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
