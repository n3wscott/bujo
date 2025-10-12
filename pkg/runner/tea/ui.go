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
	"tableflip.dev/bujo/pkg/ui/calendar"
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

// collection item for left list
type collectionItem struct {
	name        string
	resolved    string
	active      bool
	indent      bool
	folded      bool
	hasChildren bool
}

func (c collectionItem) Title() string {
	label := c.displayLabel()
	if c.active {
		return "→ " + label
	}
	return "  " + label
}
func (c collectionItem) Description() string { return "" }
func (c collectionItem) FilterValue() string {
	if c.resolved == "" || c.resolved == c.name {
		return c.name
	}
	return c.name + " " + c.resolved
}
func (c collectionItem) displayLabel() string {
	label := c.name
	if c.indent {
		if t := parseFriendlyDate(label); !t.IsZero() {
			label = fmt.Sprintf("%s - %s", t.Format("2"), t.Format("Monday"))
		}
		label = "  " + label
	} else if c.hasChildren {
		if c.folded {
			label = "▸ " + label
		} else {
			label = "▾ " + label
		}
	}
	return label
}

// entry item for right list
type entryItem struct{ e *entry.Entry }

func (it entryItem) Title() string {
	return it.e.String()
}
func (it entryItem) Description() string { return "" }
func (it entryItem) FilterValue() string { return it.e.Message }

type calendarHeaderItem struct {
	month string
	text  string
}

func (ci *calendarHeaderItem) Title() string       { return ci.text }
func (ci *calendarHeaderItem) Description() string { return "" }
func (ci *calendarHeaderItem) FilterValue() string { return ci.month }

type calendarRowItem struct {
	month    string
	week     int
	days     []int
	text     string
	rowIndex int
}

func (ci *calendarRowItem) Title() string       { return ci.text }
func (ci *calendarRowItem) Description() string { return "" }
func (ci *calendarRowItem) FilterValue() string { return ci.month }

type calendarMonthState struct {
	month     string
	monthTime time.Time
	children  []collectionItem
	headerIdx int
	weeks     []*calendarRowItem
}

// Model contains UI state
type Model struct {
	svc    *app.Service
	ctx    context.Context
	mode   mode
	action action

	focus int // 0: collections, 1: entries

	colList list.Model
	entList list.Model

	input textinput.Model

	status string

	pendingBullet  glyph.Bullet
	bulletOptions  []glyph.Bullet
	bulletIndex    int
	bulletTargetID string
	awaitingDD     bool
	lastDTime      time.Time

	termWidth           int
	termHeight          int
	verticalReserve     int
	foldState           map[string]bool
	calendarSelection   map[string]int
	calendarMonths      map[string]*calendarMonthState
	pendingResolved     string
	activeCalendarMonth string

	focusDel list.DefaultDelegate
	blurDel  list.DefaultDelegate
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

	m := Model{
		svc:               svc,
		ctx:               context.Background(),
		mode:              modeNormal,
		action:            actionNone,
		focus:             1,
		colList:           l1,
		entList:           l2,
		input:             ti,
		status:            "NORMAL: h/l move panes, j/k move, [/] fold, o add, i edit, x complete, > move, < future, : commands (:today, :q), ? help",
		pendingBullet:     glyph.Task,
		focusDel:          dFocus,
		blurDel:           dBlur,
		bulletOptions:     bulletOpts,
		foldState:         make(map[string]bool),
		calendarSelection: make(map[string]int),
		calendarMonths:    make(map[string]*calendarMonthState),
	}
	m.bulletIndex = m.findBulletIndex(m.pendingBullet)
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
	case collectionItem:
		if v.resolved != "" {
			return v.resolved
		}
		return v.name
	case *calendarRowItem:
		state := m.calendarMonths[v.month]
		if state == nil {
			return ""
		}
		day := m.calendarSelection[v.month]
		if day == 0 || !containsDay(v.days, day) {
			day = firstNonZero(v.days)
		}
		if day == 0 {
			return ""
		}
		return formatDayPath(state.monthTime, day)
	case *calendarHeaderItem:
		return v.month
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
		m.status = "ERR: " + msg.err.Error()
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
			if _, ok := m.colList.SelectedItem().(*calendarRowItem); ok {
				cmds = append(cmds, m.markCalendarSelection())
			}
		}
		if cmd := m.syncCollectionIndicators(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.markCalendarSelection())
		cmds = append(cmds, m.loadEntries())
		cmds = append(cmds, m.markCalendarSelection())
	case entriesLoadedMsg:
		m.entList.SetItems(msg.items)
	case tea.KeyPressMsg:
		switch m.mode {
		case modeHelp:
			if key := msg.String(); key == "q" || key == "esc" || key == "?" {
				m.mode = modeNormal
				m.setVerticalReserve(0)
				skipListRouting = true
			}
		case modeBulletSelect:
			switch msg.String() {
			case "esc", "q":
				m.mode = modeNormal
				m.bulletTargetID = ""
				m.setVerticalReserve(0)
				skipListRouting = true
			case "enter":
				chosen := m.bulletOptions[m.bulletIndex]
				if m.bulletTargetID == "" {
					m.pendingBullet = chosen
					m.status = fmt.Sprintf("Default bullet set to %s", chosen.Glyph().Meaning)
				} else {
					m.applySetBullet(&cmds, m.bulletTargetID, chosen)
				}
				m.mode = modeNormal
				m.bulletTargetID = ""
				m.setVerticalReserve(0)
				skipListRouting = true
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
		case modeInsert:
			switch msg.String() {
			case "enter":
				input := strings.TrimSpace(m.input.Value())
				switch m.action {
				case actionAdd:
					col := m.selectedCollection()
					if col != "" && input != "" {
						if _, err := m.svc.Add(m.ctx, col, m.pendingBullet, input, glyph.None); err != nil {
							cmds = append(cmds, func() tea.Msg { return errMsg{err} })
						} else {
							m.status = "Added"
						}
					}
				case actionEdit:
					if it := m.currentEntry(); it != nil {
						if _, err := m.svc.Edit(m.ctx, it.e.ID, input); err != nil {
							cmds = append(cmds, func() tea.Msg { return errMsg{err} })
						} else {
							m.status = "Edited"
						}
					}
				case actionMove:
					if it := m.currentEntry(); it != nil && input != "" {
						if _, err := m.svc.Move(m.ctx, it.e.ID, input); err != nil {
							cmds = append(cmds, func() tea.Msg { return errMsg{err} })
						} else {
							m.status = "Moved"
						}
					}
				}
				m.mode = modeNormal
				m.action = actionNone
				m.input.Reset()
				m.input.Blur()
				cmds = append(cmds, m.refreshAll())
				skipListRouting = true
			case "esc", "q":
				prevAction := m.action
				m.mode = modeNormal
				m.action = actionNone
				m.input.Reset()
				m.input.Blur()
				switch prevAction {
				case actionAdd:
					m.status = "Add cancelled"
				case actionEdit:
					m.status = "Edit cancelled"
				case actionMove:
					m.status = "Move cancelled"
				default:
					m.status = "Cancelled"
				}
				skipListRouting = true
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				cmds = append(cmds, cmd)
			}
		case modeCommand:
			// Reserved for future ':' commands
			switch msg.String() {
			case "enter":
				input := strings.TrimSpace(m.input.Value())
				switch input {
				case "q", "quit", "exit":
					cmds = append(cmds, tea.Quit)
				case "today":
					if cmd := m.selectToday(); cmd != nil {
						cmds = append(cmds, cmd)
					}
				case "":
					// nothing
				default:
					m.status = fmt.Sprintf("Unknown command: %s", input)
				}
				m.mode = modeNormal
				m.input.Reset()
				m.input.Blur()
				m.setVerticalReserve(0)
				skipListRouting = true
			case "esc":
				m.mode = modeNormal
				m.input.Reset()
				m.input.Blur()
				m.status = "Command cancelled"
				m.setVerticalReserve(0)
				skipListRouting = true
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				cmds = append(cmds, cmd)
			}
		case modeNormal:
			// Vim-style navigation and commands
			switch msg.String() {
			case ":":
				m.enterCommandMode(&cmds)
				skipListRouting = true

			// pane focus
			case "esc":
				if m.focus == 1 {
					m.focus = 0
					m.updateFocusHeaders()
					cmds = append(cmds, m.loadEntries())
					if cmd := m.syncCollectionIndicators(); cmd != nil {
						cmds = append(cmds, cmd)
					}
					skipListRouting = true
				}
			case "h", "left":
				if m.focus == 0 {
					if cmd := m.moveCalendarCursor(-1, 0); cmd != nil {
						cmds = append(cmds, cmd)
						skipListRouting = true
						break
					}
				}
				m.focus = 0
				m.updateFocusHeaders()
				cmds = append(cmds, m.loadEntries())
				if cmd := m.syncCollectionIndicators(); cmd != nil {
					cmds = append(cmds, cmd)
				}
				skipListRouting = true
			case "l", "right":
				if m.focus == 0 {
					if cmd := m.moveCalendarCursor(1, 0); cmd != nil {
						cmds = append(cmds, cmd)
						skipListRouting = true
						break
					}
				}
				m.focus = 1
				m.updateFocusHeaders()
				cmds = append(cmds, m.loadEntries())
				if cmd := m.syncCollectionIndicators(); cmd != nil {
					cmds = append(cmds, cmd)
				}
				skipListRouting = true
			case "enter":
				if m.focus == 0 {
					if cmd := m.markCalendarSelection(); cmd != nil {
						cmds = append(cmds, cmd)
					}
					m.focus = 1
					m.updateFocusHeaders()
					if cmd := m.syncCollectionIndicators(); cmd != nil {
						cmds = append(cmds, cmd)
					}
					skipListRouting = true
					break
				}
				m.focus = 1
				m.updateFocusHeaders()
				cmds = append(cmds, m.loadEntries())
				if cmd := m.syncCollectionIndicators(); cmd != nil {
					cmds = append(cmds, cmd)
				}
				skipListRouting = true

			// movement
			case "j", "down":
				handled := false
				if m.focus == 0 {
					if cmd := m.moveCalendarCursor(0, 1); cmd != nil {
						cmds = append(cmds, cmd)
						skipListRouting = true
						break
					}
				}
				if m.focus == 0 {
					m.colList.CursorDown()
					if cmd := m.syncCollectionIndicators(); cmd != nil {
						cmds = append(cmds, cmd)
					}
					cmds = append(cmds, m.loadEntries())
					m.updateActiveMonthFromSelection(false, &cmds)
					handled = true
				} else {
					m.entList.CursorDown()
					handled = true
				}
				if handled {
					skipListRouting = true
				}
			case "k", "up":
				handled := false
				if m.focus == 0 {
					if cmd := m.moveCalendarCursor(0, -1); cmd != nil {
						cmds = append(cmds, cmd)
						skipListRouting = true
						break
					}
				}
				if m.focus == 0 {
					m.colList.CursorUp()
					if cmd := m.syncCollectionIndicators(); cmd != nil {
						cmds = append(cmds, cmd)
					}
					cmds = append(cmds, m.loadEntries())
					m.updateActiveMonthFromSelection(false, &cmds)
					handled = true
				} else {
					m.entList.CursorUp()
					handled = true
				}
				if handled {
					skipListRouting = true
				}
			case "g":
				// support gg: handled by awaitingDD-style small window; simplest just go top on single g
				if m.focus == 0 {
					m.colList.Select(0)
					if cmd := m.syncCollectionIndicators(); cmd != nil {
						cmds = append(cmds, cmd)
					}
					m.updateActiveMonthFromSelection(false, &cmds)
					cmds = append(cmds, m.markCalendarSelection())
				} else {
					m.entList.Select(0)
				}
			case "G":
				if m.focus == 0 {
					m.colList.Select(len(m.colList.Items()) - 1)
					if cmd := m.syncCollectionIndicators(); cmd != nil {
						cmds = append(cmds, cmd)
					}
					m.updateActiveMonthFromSelection(false, &cmds)
					cmds = append(cmds, m.markCalendarSelection())
				} else {
					m.entList.Select(len(m.entList.Items()) - 1)
				}
			case "[":
				if m.focus == 0 {
					collapse := true
					if cmd := m.toggleFoldCurrent(&collapse); cmd != nil {
						cmds = append(cmds, cmd)
					}
					skipListRouting = true
				}
			case "]":
				if m.focus == 0 {
					expand := false
					if cmd := m.toggleFoldCurrent(&expand); cmd != nil {
						cmds = append(cmds, cmd)
					}
					skipListRouting = true
				}

			// add
			case "o", "O":
				m.mode = modeInsert
				m.action = actionAdd
				m.input.Placeholder = "New item message"
				m.input.SetValue("")
				if cmd := m.input.Focus(); cmd != nil {
					cmds = append(cmds, cmd)
				}
				cmds = append(cmds, textinput.Blink)

			// edit
			case "i":
				if it := m.currentEntry(); it != nil {
					m.mode = modeInsert
					m.action = actionEdit
					m.input.Placeholder = "Edit message"
					m.input.SetValue(it.e.Message)
					m.input.CursorEnd()
					if cmd := m.input.Focus(); cmd != nil {
						cmds = append(cmds, cmd)
					}
					cmds = append(cmds, textinput.Blink)
				}

			// complete
			case "x":
				if it := m.currentEntry(); it != nil {
					if _, err := m.svc.Complete(m.ctx, it.e.ID); err != nil {
						cmds = append(cmds, func() tea.Msg { return errMsg{err} })
					} else {
						m.status = "Completed"
						cmds = append(cmds, m.refreshAll())
					}
				}

			// strike: treat single d as strike for now, optional double-d logic
			case "d":
				if it := m.currentEntry(); it != nil {
					if m.awaitingDD && time.Since(m.lastDTime) < 600*time.Millisecond {
						if _, err := m.svc.Strike(m.ctx, it.e.ID); err != nil {
							cmds = append(cmds, func() tea.Msg { return errMsg{err} })
						} else {
							m.status = "Struck"
							cmds = append(cmds, m.refreshAll())
						}
						m.awaitingDD = false
					} else {
						m.awaitingDD = true
						m.lastDTime = time.Now()
					}
				}

			// move
			case ">":
				if m.currentEntry() != nil {
					m.mode = modeInsert
					m.action = actionMove
					m.input.Placeholder = "Move to collection"
					m.input.SetValue("")
					if cmd := m.input.Focus(); cmd != nil {
						cmds = append(cmds, cmd)
					}
					cmds = append(cmds, textinput.Blink)
				}
			case "<":
				if it := m.currentEntry(); it != nil {
					if _, err := m.svc.Move(m.ctx, it.e.ID, "Future"); err != nil {
						cmds = append(cmds, func() tea.Msg { return errMsg{err} })
					} else {
						m.status = "Moved to Future"
						cmds = append(cmds, m.refreshAll())
					}
				}

			// bullets
			case "t":
				m.pendingBullet = glyph.Task
			case "n":
				m.pendingBullet = glyph.Note
			case "e":
				m.pendingBullet = glyph.Event
			case "b":
				var target string
				var current glyph.Bullet = m.pendingBullet
				if m.focus == 1 {
					if it := m.currentEntry(); it != nil {
						target = it.e.ID
						current = it.e.Bullet
					}
				}
				m.enterBulletSelect(target, current)
				skipListRouting = true
			// set bullet on selected entry
			case "T":
				if it := m.currentEntry(); it != nil {
					m.applySetBullet(&cmds, it.e.ID, glyph.Task)
				}
			case "N":
				if it := m.currentEntry(); it != nil {
					m.applySetBullet(&cmds, it.e.ID, glyph.Note)
				}
			case "E":
				if it := m.currentEntry(); it != nil {
					m.applySetBullet(&cmds, it.e.ID, glyph.Event)
				}

			// signifiers
			case "*":
				if it := m.currentEntry(); it != nil {
					m.applyToggleSig(&cmds, it.e.ID, glyph.Priority)
				}
			case "!":
				if it := m.currentEntry(); it != nil {
					m.applyToggleSig(&cmds, it.e.ID, glyph.Inspiration)
				}
			case "?":
				m.mode = modeHelp
				m.setVerticalReserve(3)
				skipListRouting = true

			// quit/refresh
			case "r":
				cmds = append(cmds, m.refreshAll())
			case "q":
				m.status = "Use :q or :exit to quit"
				skipListRouting = true
			}
		}
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

func (m *Model) applySetBullet(cmds *[]tea.Cmd, id string, b glyph.Bullet) {
	if _, err := m.svc.SetBullet(m.ctx, id, b); err != nil {
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
	} else {
		m.status = "Bullet updated"
		*cmds = append(*cmds, m.refreshAll())
	}
}

func (m *Model) applyToggleSig(cmds *[]tea.Cmd, id string, s glyph.Signifier) {
	if _, err := m.svc.ToggleSignifier(m.ctx, id, s); err != nil {
		*cmds = append(*cmds, func() tea.Msg { return errMsg{err} })
	} else {
		m.status = "Signifier toggled"
		*cmds = append(*cmds, m.refreshAll())
	}
}

// View renders two lists and optional input/help overlays
func (m Model) View() string {
	left := m.colList.View()
	m.updateEntriesTitle()
	right := m.entList.View()
	gap := lipgloss.NewStyle().Padding(0, 1).Render
	modeStr := map[mode]string{modeNormal: "NORMAL", modeInsert: "INSERT", modeCommand: "CMD", modeHelp: "HELP"}[m.mode]
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(fmt.Sprintf("[%s] %s (add bullet: %s)", modeStr, m.status, m.pendingBullet.String()))

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, gap(" "), right)

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
		body += "\n\n" + prompt + m.input.View()
	}
	if m.mode == modeCommand {
		body += "\n\n:" + m.input.View()
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
		body += "\n\n" + panelStyle.Render(strings.Join(lines, "\n"))
	}
	if m.mode == modeHelp {
		help := "Keys: ←/→ switch panes, ↑/↓ move, gg/G top/bottom, [/] fold, o add, i edit, x complete, dd strike, > move, < future, t/n/e set add-bullet, T/N/E set on item, */!/?: toggle signifiers, :q quit, :today jump"
		body += "\n\n" + lipgloss.NewStyle().Italic(true).Render(help)
	}

	return body + "\n\n" + status
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
	m.mode = modeBulletSelect
	m.bulletTargetID = targetID
	m.bulletIndex = m.findBulletIndex(current)
	reserve := len(m.bulletOptions) + 5
	m.setVerticalReserve(reserve)
	if targetID == "" {
		m.status = "Choose default bullet for new entries"
	} else {
		m.status = "Choose bullet for selected entry"
	}
}

func (m *Model) setVerticalReserve(lines int) {
	if lines < 0 {
		lines = 0
	}
	if m.verticalReserve == lines {
		return
	}
	m.verticalReserve = lines
	m.applySizes()
}

func (m *Model) enterCommandMode(cmds *[]tea.Cmd) {
	m.mode = modeCommand
	m.input.Reset()
	m.input.Placeholder = "command"
	m.input.CursorEnd()
	m.setVerticalReserve(2)
	if cmd := m.input.Focus(); cmd != nil {
		*cmds = append(*cmds, cmd)
	}
	*cmds = append(*cmds, textinput.Blink)
	m.status = "COMMAND: :q to quit, :today jump to Today"
}

func (m *Model) selectToday() tea.Cmd {
	month, todayDay, todayResolved := todayLabels()
	m.foldState[month] = false
	items := m.colList.Items()
	var updateCmds []tea.Cmd
	targetIdx := -1
	for i, it := range items {
		ci, ok := it.(collectionItem)
		if !ok {
			continue
		}
		if ci.name == todayMetaName {
			targetIdx = i
			if ci.resolved != todayResolved {
				ci.resolved = todayResolved
				if cmd := m.colList.SetItem(i, ci); cmd != nil {
					updateCmds = append(updateCmds, cmd)
				}
			}
			break
		}
	}
	if targetIdx == -1 {
		ci := collectionItem{name: todayMetaName, resolved: todayResolved}
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
	m.setVerticalReserve(0)
	m.status = fmt.Sprintf("Selected Today (%s)", todayDay)

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
	case collectionItem:
		if v.indent {
			if v.resolved == "" {
				return nil
			}
			parts := strings.SplitN(v.resolved, "/", 2)
			if len(parts) == 0 {
				return nil
			}
			key = parts[0]
		} else {
			if !v.hasChildren {
				return nil
			}
			key = v.resolved
			if key == "" {
				key = v.name
			}
		}
	case *calendarRowItem:
		key = v.month
	case *calendarHeaderItem:
		key = v.month
	default:
		return nil
	}
	if key == "" {
		return nil
	}
	current := m.foldState[key]
	var desired bool
	if explicit == nil {
		desired = !current
	} else {
		desired = *explicit
		if current == desired {
			return nil
		}
	}
	m.foldState[key] = desired
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
		ci, ok := it.(collectionItem)
		if !ok {
			continue
		}
		wantActive := i == activeIdx
		if ci.active == wantActive {
			continue
		}
		ci.active = wantActive
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
	ci, ok := sel.(collectionItem)
	if !ok {
		return ""
	}
	if ci.resolved != "" {
		return ci.resolved
	}
	return ci.name
}

func indexForResolved(items []list.Item, resolved string) int {
	for i, it := range items {
		switch v := it.(type) {
		case collectionItem:
			if v.resolved == resolved || (v.resolved == "" && v.name == resolved) {
				return i
			}
		case *calendarHeaderItem:
			if resolved == v.month {
				return i
			}
		case *calendarRowItem:
			if strings.HasPrefix(resolved, v.month+"/") {
				if day := dayFromPath(resolved); day > 0 && containsDay(v.days, day) {
					return i
				}
			}
		}
	}
	return -1
}

func indexForName(items []list.Item, name string) int {
	for i, it := range items {
		ci, ok := it.(collectionItem)
		if !ok {
			continue
		}
		if ci.name == name {
			return i
		}
	}
	return -1
}

func (m *Model) buildCollectionItems(cols []string, currentResolved string, now time.Time) []list.Item {
	if m.calendarSelection == nil {
		m.calendarSelection = make(map[string]int)
	}
	if m.calendarMonths == nil {
		m.calendarMonths = make(map[string]*calendarMonthState)
	}
	for k := range m.calendarMonths {
		delete(m.calendarMonths, k)
	}

	todayMonth := now.Format("January 2006")
	todayResolved := fmt.Sprintf("%s/%s", todayMonth, now.Format("January 2, 2006"))
	todayItem := collectionItem{name: todayMetaName, resolved: todayResolved}

	type monthEntry struct {
		name string
		time time.Time
		base collectionItem
	}

	monthChildren := make(map[string][]collectionItem)
	monthEntries := make(map[string]*monthEntry)
	otherChildren := make(map[string][]collectionItem)
	otherBases := make(map[string]collectionItem)
	otherOrder := make([]string, 0)

	addOtherBase := func(name string, item collectionItem) {
		if _, ok := otherBases[name]; !ok {
			otherBases[name] = item
			otherOrder = append(otherOrder, name)
		}
	}

	for _, raw := range cols {
		parts := strings.SplitN(raw, "/", 2)
		if len(parts) == 2 {
			parent, child := parts[0], parts[1]
			if t, isMonth := parseMonth(parent); isMonth {
				monthChildren[parent] = append(monthChildren[parent], collectionItem{name: child, resolved: raw, indent: true})
				if _, ok := monthEntries[parent]; !ok {
					monthEntries[parent] = &monthEntry{name: parent, time: t}
				}
			} else {
				otherChildren[parent] = append(otherChildren[parent], collectionItem{name: child, resolved: raw, indent: true})
				addOtherBase(parent, collectionItem{name: parent, resolved: parent})
			}
			continue
		}

		if t, isMonth := parseMonth(raw); isMonth {
			entry := monthEntries[raw]
			if entry == nil {
				entry = &monthEntry{name: raw, time: t}
				monthEntries[raw] = entry
			} else if entry.time.IsZero() {
				entry.time = t
			}
			entry.base = collectionItem{name: raw, resolved: raw}
			continue
		}

		addOtherBase(raw, collectionItem{name: raw, resolved: raw})
	}

	if _, ok := monthEntries[todayMonth]; !ok {
		if t, isMonth := parseMonth(todayMonth); isMonth {
			monthEntries[todayMonth] = &monthEntry{
				name: todayMonth,
				time: t,
				base: collectionItem{name: todayMonth, resolved: todayMonth},
			}
		}
	}

	months := make([]*monthEntry, 0, len(monthEntries))
	for name, entry := range monthEntries {
		if entry.base.name == "" {
			entry.base = collectionItem{name: name, resolved: name}
		}
		if entry.time.IsZero() {
			if t, ok := parseMonth(name); ok {
				entry.time = t
			}
		}
		months = append(months, entry)
	}
	sort.Slice(months, func(i, j int) bool {
		ti, tj := months[i].time, months[j].time
		switch {
		case ti.Equal(tj):
			return months[i].name > months[j].name
		case ti.IsZero():
			return false
		case tj.IsZero():
			return true
		default:
			return ti.After(tj)
		}
	})

	result := make([]list.Item, 0, len(cols)+16)
	result = append(result, todayItem)

	appendCollection := func(base collectionItem, children []collectionItem, monthTime time.Time, isMonth bool) {
		key := base.resolved
		if key == "" {
			key = base.name
		}
		if isMonth {
			if _, ok := m.foldState[key]; !ok {
				m.foldState[key] = base.name != todayMonth && base.resolved != todayMonth
			}
		}
		base.hasChildren = isMonth || len(children) > 0
		base.folded = m.foldState[key]
		result = append(result, base)

		if !isMonth {
			if len(children) > 0 {
				sortCollectionChildren(children)
				if !base.folded {
					for _, child := range children {
						result = append(result, child)
					}
				}
			}
			return
		}

		state := &calendarMonthState{
			month:     base.resolved,
			monthTime: monthTime,
			children:  append([]collectionItem(nil), children...),
			headerIdx: -1,
		}
		m.calendarMonths[base.resolved] = state

		selected := m.calendarSelection[base.resolved]
		if selected == 0 {
			selected = m.defaultSelectedDay(base.resolved, monthTime, children, currentResolved, now)
			if selected > 0 {
				m.calendarSelection[base.resolved] = selected
			}
		}

		if base.folded {
			return
		}

		selectedForRender := selected
		if m.activeCalendarMonth != base.resolved {
			selectedForRender = 0
		}

		header, weeks := renderCalendarRows(base.resolved, monthTime, children, selectedForRender, now, defaultCalendarOptions())
		if header == nil {
			return
		}
		state.headerIdx = len(result)
		result = append(result, header)
		for _, week := range weeks {
			week.rowIndex = len(result)
			result = append(result, week)
		}
		state.weeks = weeks
	}

	for _, entry := range months {
		appendCollection(entry.base, monthChildren[entry.name], entry.time, true)
	}

	for _, name := range otherOrder {
		base := otherBases[name]
		children := otherChildren[name]
		appendCollection(base, children, time.Time{}, false)
	}

	return result
}

func sortCollectionChildren(children []collectionItem) {
	sort.SliceStable(children, func(i, j int) bool {
		ti := parseFriendlyDate(children[i].name)
		tj := parseFriendlyDate(children[j].name)
		if !ti.IsZero() && !tj.IsZero() {
			return ti.Before(tj)
		}
		if ti.IsZero() != tj.IsZero() {
			return !ti.IsZero()
		}
		return strings.Compare(children[i].name, children[j].name) < 0
	})
}

func parseFriendlyDate(s string) time.Time {
	layouts := []string{"January 2, 2006", "2006-01-02"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
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

func parseMonth(name string) (time.Time, bool) {
	if t, err := time.Parse("January 2006", name); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func renderCalendarRows(month string, monthTime time.Time, children []collectionItem, selectedDay int, now time.Time, opts calendar.Options) (*calendarHeaderItem, []*calendarRowItem) {
	header := &calendarHeaderItem{
		month: month,
		text:  "  " + opts.HeaderStyle.Render("Su Mo Tu We Th Fr Sa"),
	}

	entryDays := make(map[int]bool)
	for _, child := range children {
		if day := dayNumberFromName(monthTime, child.name); day > 0 {
			entryDays[day] = true
		}
	}

	todayDay := 0
	if monthTime.Year() == now.Year() && monthTime.Month() == now.Month() {
		todayDay = now.Day()
	}

	first := time.Date(monthTime.Year(), monthTime.Month(), 1, 0, 0, 0, 0, monthTime.Location())
	offset := int(first.Weekday())
	daysInMonth := daysIn(monthTime)
	totalCells := offset + daysInMonth
	rowsCount := (totalCells + 6) / 7

	weeks := make([]*calendarRowItem, 0, rowsCount)
	for row := 0; row < rowsCount; row++ {
		var cells []string
		weekDays := make([]int, 7)
		for col := 0; col < 7; col++ {
			cellIdx := row*7 + col
			day := cellIdx - offset + 1
			if day < 1 || day > daysInMonth {
				cells = append(cells, opts.EmptyStyle.Render("  "))
				weekDays[col] = 0
				continue
			}
			style := opts.EmptyStyle
			if entryDays[day] {
				style = opts.EntryStyle
			}
			if day == todayDay {
				style = style.Inherit(opts.TodayStyle)
			}
			if selectedDay > 0 && day == selectedDay {
				style = style.Inherit(opts.SelectedStyle)
			}
			cells = append(cells, style.Render(fmt.Sprintf("%2d", day)))
			weekDays[col] = day
		}
		text := "  " + strings.Join(cells, " ")
		weekItem := &calendarRowItem{
			month: month,
			week:  row,
			days:  weekDays,
			text:  text,
		}
		weeks = append(weeks, weekItem)
	}

	return header, weeks
}

func buildDayMetas(month time.Time, children []collectionItem, currentResolved string, now time.Time) []calendar.Day {
	daysInMonth := time.Date(month.Year(), month.Month()+1, 0, 0, 0, 0, 0, month.Location()).Day()
	entries := make(map[int]bool)
	selectedDay := -1
	for _, child := range children {
		t := parseFriendlyDate(child.name)
		if t.IsZero() {
			continue
		}
		entries[t.Day()] = true
		if child.resolved == currentResolved {
			selectedDay = t.Day()
		}
	}

	metas := make([]calendar.Day, 0, daysInMonth)
	for d := 1; d <= daysInMonth; d++ {
		meta := calendar.Day{Day: d, HasEntry: entries[d]}
		if now.Year() == month.Year() && now.Month() == month.Month() && now.Day() == d {
			meta.IsToday = true
		}
		if d == selectedDay {
			meta.IsSelected = true
		}
		metas = append(metas, meta)
	}
	return metas
}

func defaultCalendarOptions() calendar.Options {
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Bold(true)
	empty := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	entry := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	today := lipgloss.NewStyle().Underline(true)
	selected := lipgloss.NewStyle().Background(lipgloss.Color("63")).Foreground(lipgloss.Color("0"))
	return calendar.Options{
		HeaderStyle:   header,
		EmptyStyle:    empty,
		EntryStyle:    entry,
		TodayStyle:    today,
		SelectedStyle: selected,
		ShowHeader:    true,
	}
}

func (m *Model) ensureCollectionSelection(direction int) {
	idx := m.colList.Index()
	items := m.colList.Items()
	if idx < 0 || idx >= len(items) {
		return
	}
	if direction >= 0 {
		for idx < len(items) && isCalendar(items[idx]) {
			m.colList.CursorDown()
			idx = m.colList.Index()
		}
		if idx >= len(items) {
			for idx >= 0 && isCalendar(items[idx]) {
				m.colList.CursorUp()
				idx = m.colList.Index()
			}
		}
	} else {
		for idx >= 0 && isCalendar(items[idx]) {
			m.colList.CursorUp()
			idx = m.colList.Index()
		}
	}
}

func isCalendar(it list.Item) bool {
	switch it.(type) {
	case *calendarRowItem, *calendarHeaderItem:
		return true
	default:
		return false
	}
}

func (m *Model) markCalendarSelection() tea.Cmd {
	sel := m.colList.SelectedItem()
	switch v := sel.(type) {
	case *calendarHeaderItem:
		state := m.calendarMonths[v.month]
		if state == nil || len(state.weeks) == 0 {
			return nil
		}
		m.colList.Select(state.weeks[0].rowIndex)
		return m.markCalendarSelection()
	case *calendarRowItem:
		state := m.calendarMonths[v.month]
		if state == nil {
			return nil
		}
		day := m.calendarSelection[v.month]
		if day == 0 || !containsDay(v.days, day) {
			day = firstNonZero(v.days)
		}
		if day == 0 {
			return nil
		}
		m.calendarSelection[v.month] = day
		m.pendingResolved = formatDayPath(state.monthTime, day)
		var cmds []tea.Cmd
		m.applyActiveCalendarMonth(v.month, true, &cmds)
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
	case *calendarRowItem:
		month = v.month
	case *calendarHeaderItem:
		month = v.month
	default:
		return nil
	}

	state := m.calendarMonths[month]
	if state == nil || len(state.weeks) == 0 {
		return nil
	}

	selected := m.calendarSelection[month]
	if selected == 0 {
		selected = m.defaultSelectedDay(month, state.monthTime, state.children, m.pendingResolved, time.Now())
		if selected == 0 {
			selected = firstNonZero(state.weeks[0].days)
		}
	}
	if selected == 0 {
		return nil
	}

	newDay := selected + dx + dy*7
	daysInMonth := daysIn(state.monthTime)
	if newDay < 1 {
		newDay = 1
	}
	if newDay > daysInMonth {
		newDay = daysInMonth
	}
	if newDay == selected {
		return nil
	}

	m.calendarSelection[month] = newDay
	m.pendingResolved = formatDayPath(state.monthTime, newDay)

	var cmds []tea.Cmd
	m.applyActiveCalendarMonth(month, true, &cmds)
	if week := m.findWeekForDay(month, newDay); week != nil {
		if m.colList.Index() != week.rowIndex {
			m.colList.Select(week.rowIndex)
		}
	}
	cmds = append(cmds, m.loadEntries())
	return tea.Batch(cmds...)
}

func (m *Model) findWeekForDay(month string, day int) *calendarRowItem {
	state := m.calendarMonths[month]
	if state == nil {
		return nil
	}
	for _, week := range state.weeks {
		if containsDay(week.days, day) {
			return week
		}
	}
	return nil
}

func (m *Model) refreshCalendarMonth(month string) tea.Cmd {
	state := m.calendarMonths[month]
	if state == nil || state.headerIdx < 0 {
		return nil
	}
	selected := m.calendarSelection[month]
	if m.activeCalendarMonth != month {
		selected = 0
	}

	header, weeks := renderCalendarRows(month, state.monthTime, state.children, selected, time.Now(), defaultCalendarOptions())
	if header == nil {
		return nil
	}

	var cmds []tea.Cmd
	headerIdx := state.headerIdx
	if headerIdx >= len(m.colList.Items()) {
		return nil
	}
	if cmd := m.colList.SetItem(headerIdx, header); cmd != nil {
		cmds = append(cmds, cmd)
	}

	oldCount := len(state.weeks)
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
		week.rowIndex = idx
		if cmd := m.colList.SetItem(idx, week); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	state.weeks = weeks
	m.calendarMonths[month] = state
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) applyActiveCalendarMonth(month string, force bool, cmds *[]tea.Cmd) {
	prev := m.activeCalendarMonth
	changed := prev != month
	if changed {
		m.activeCalendarMonth = month
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
	case *calendarRowItem:
		m.applyActiveCalendarMonth(v.month, force, cmds)
	case *calendarHeaderItem:
		m.applyActiveCalendarMonth(v.month, force, cmds)
	default:
		m.applyActiveCalendarMonth("", false, cmds)
	}
}

func (m *Model) defaultSelectedDay(month string, monthTime time.Time, children []collectionItem, currentResolved string, now time.Time) int {
	if strings.HasPrefix(currentResolved, month+"/") {
		if day := dayFromPath(currentResolved); day > 0 {
			return day
		}
	}
	if monthTime.Year() == now.Year() && monthTime.Month() == now.Month() {
		return now.Day()
	}
	for _, child := range children {
		if day := dayNumberFromName(monthTime, child.name); day > 0 {
			return day
		}
	}
	return 0
}

func dayNumberFromName(monthTime time.Time, name string) int {
	t, err := time.Parse("January 2, 2006", name)
	if err != nil {
		return 0
	}
	if t.Year() != monthTime.Year() || t.Month() != monthTime.Month() {
		return 0
	}
	return t.Day()
}

func dayFromPath(path string) int {
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return 0
	}
	t, err := time.Parse("January 2, 2006", parts[1])
	if err != nil {
		return 0
	}
	return t.Day()
}

func formatDayPath(monthTime time.Time, day int) string {
	dt := time.Date(monthTime.Year(), monthTime.Month(), day, 0, 0, 0, 0, monthTime.Location())
	return fmt.Sprintf("%s/%s", monthTime.Format("January 2006"), dt.Format("January 2, 2006"))
}

func firstNonZero(days []int) int {
	for _, d := range days {
		if d > 0 {
			return d
		}
	}
	return 0
}

func containsDay(days []int, target int) bool {
	for _, d := range days {
		if d == target {
			return true
		}
	}
	return false
}

func daysIn(month time.Time) int {
	first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
	return first.AddDate(0, 1, -1).Day()
}
