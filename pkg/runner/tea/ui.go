package teaui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
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

// collection item for left list
type collectionItem struct{ name string }

func (c collectionItem) Title() string       { return c.name }
func (c collectionItem) Description() string { return "" }
func (c collectionItem) FilterValue() string { return c.name }

// entry item for right list
type entryItem struct{ e *entry.Entry }

func (it entryItem) Title() string {
	return it.e.String()
}
func (it entryItem) Description() string { return "" }
func (it entryItem) FilterValue() string { return it.e.Message }

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

	termWidth  int
	termHeight int

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
	l1.Title = "Collections"
	l1.SetShowHelp(false)
	l1.SetShowStatusBar(false)

	l2 := list.New([]list.Item{}, dFocus, 80, 20)
	l2.Title = "Entries"
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
		svc:           svc,
		ctx:           context.Background(),
		mode:          modeNormal,
		action:        actionNone,
		focus:         1,
		colList:       l1,
		entList:       l2,
		input:         ti,
		status:        "NORMAL: h/l move panes, j/k move, o add, i edit, x complete, > move, < future, : commands, ? help",
		pendingBullet: glyph.Task,
		focusDel:      dFocus,
		blurDel:       dBlur,
		bulletOptions: bulletOpts,
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
	return func() tea.Msg {
		cols, err := m.svc.Collections(m.ctx)
		if err != nil {
			return errMsg{err}
		}
		items := make([]list.Item, 0, len(cols))
		for _, c := range cols {
			items = append(items, collectionItem{name: c})
		}
		return collectionsLoadedMsg{items}
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
	return sel.(collectionItem).name
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
		m.colList.SetItems(msg.items)
		if len(msg.items) > 0 && m.colList.Index() < 0 {
			m.colList.Select(0)
		}
		cmds = append(cmds, m.loadEntries())
	case entriesLoadedMsg:
		m.entList.SetItems(msg.items)
	case tea.KeyPressMsg:
		switch m.mode {
		case modeHelp:
			if key := msg.String(); key == "q" || key == "esc" || key == "?" {
				m.mode = modeNormal
				skipListRouting = true
			}
		case modeBulletSelect:
			switch msg.String() {
			case "esc", "q":
				m.mode = modeNormal
				m.bulletTargetID = ""
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
				case "":
					// nothing
				default:
					m.status = fmt.Sprintf("Unknown command: %s", input)
				}
				m.mode = modeNormal
				m.input.Reset()
				m.input.Blur()
				skipListRouting = true
			case "esc":
				m.mode = modeNormal
				m.input.Reset()
				m.input.Blur()
				m.status = "Command cancelled"
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
			case "h", "left":
				m.focus = 0
				m.updateFocusHeaders()
				cmds = append(cmds, m.loadEntries())
				skipListRouting = true
			case "l", "right":
				m.focus = 1
				m.updateFocusHeaders()
				// ensure entries reflect the currently selected collection
				cmds = append(cmds, m.loadEntries())
				skipListRouting = true

			// movement
			case "j":
				if m.focus == 0 {
					m.colList.CursorDown()
					cmds = append(cmds, m.loadEntries())
				} else {
					m.entList.CursorDown()
				}
			case "k":
				if m.focus == 0 {
					m.colList.CursorUp()
					cmds = append(cmds, m.loadEntries())
				} else {
					m.entList.CursorUp()
				}
			case "g":
				// support gg: handled by awaitingDD-style small window; simplest just go top on single g
				if m.focus == 0 {
					m.colList.Select(0)
					cmds = append(cmds, m.loadEntries())
				} else {
					m.entList.Select(0)
				}
			case "G":
				if m.focus == 0 {
					m.colList.Select(len(m.colList.Items()) - 1)
					cmds = append(cmds, m.loadEntries())
				} else {
					m.entList.Select(len(m.entList.Items()) - 1)
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
				if m.mode == modeNormal {
					m.mode = modeHelp
				} else {
					m.mode = modeNormal
				}

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
			if newSel := m.selectedCollection(); newSel != prev {
				cmds = append(cmds, m.loadEntries())
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
		help := "Keys: ←/→ switch panes, ↑/↓ move, gg/G top/bottom, o add, i edit, x complete, dd strike, > move, < future, t/n/e set add-bullet, T/N/E set on item, */!/?: toggle signifiers, :q quit"
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
	height := m.termHeight - 4
	if height < 5 {
		height = 5
	}
	m.colList.SetSize(left, height)
	m.entList.SetSize(right, height)
}

// updateFocusHeaders updates pane titles to reflect which pane is focused.
func (m *Model) updateFocusHeaders() {
	// Use fixed-width 2-char prefix to avoid layout shift when focus changes.
	const on = "» "
	const off = "  "
	if m.focus == 0 {
		m.colList.Title = on + "Collections"
		m.entList.Title = off + "Entries"
		m.colList.SetDelegate(m.focusDel)
		m.entList.SetDelegate(m.blurDel)
	} else {
		m.colList.Title = off + "Collections"
		m.entList.Title = on + "Entries"
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
	if targetID == "" {
		m.status = "Choose default bullet for new entries"
	} else {
		m.status = "Choose bullet for selected entry"
	}
}

func (m *Model) enterCommandMode(cmds *[]tea.Cmd) {
	m.mode = modeCommand
	m.input.Reset()
	m.input.Placeholder = "command"
	m.input.CursorEnd()
	if cmd := m.input.Focus(); cmd != nil {
		*cmds = append(*cmds, cmd)
	}
	*cmds = append(*cmds, textinput.Blink)
	m.status = "COMMAND: type :q or :exit to quit"
}
