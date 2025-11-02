package journal

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	cachepkg "tableflip.dev/bujo/pkg/tui/cache"
	"tableflip.dev/bujo/pkg/tui/components/addtask"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/components/collectionnav"
	"tableflip.dev/bujo/pkg/tui/events"
)

// FocusPane identifies which child currently owns keyboard focus.
type FocusPane int

const (
	// FocusNav indicates the navigation pane owns focus.
	FocusNav FocusPane = iota
	// FocusDetail indicates the detail pane owns focus.
	FocusDetail
)

const (
	minNavWidth = 22
	maxNavWidth = 25
	gutterWidth = 1
)

// Model composes the collection nav and detail panes side by side.
type Model struct {
	nav    *collectionnav.Model
	detail *collectiondetail.Model
	cache  *cachepkg.Cache

	width       int
	height      int
	navWidth    int
	detailWidth int

	focus    FocusPane
	navID    events.ComponentID
	detailID events.ComponentID
	addID    events.ComponentID

	add       *addtask.Model
	addOrigin FocusPane
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// NewModel builds a journal component from the provided children and shared cache.
func NewModel(nav *collectionnav.Model, detail *collectiondetail.Model, cache *cachepkg.Cache) *Model {
	m := &Model{
		nav:    nav,
		detail: detail,
		cache:  cache,
		focus:  FocusNav,
		addID:  events.ComponentID("journal-addtask"),
	}
	if nav != nil {
		m.navID = nav.ID()
	}
	if detail != nil {
		m.detailID = detail.ID()
		if m.navID != "" {
			detail.SetSourceNav(m.navID)
		}
	}
	return m
}

// SetSize splits the available area between nav and detail.
func (m *Model) SetSize(width, height int) {
	if width < minNavWidth+gutterWidth+1 {
		width = minNavWidth + gutterWidth + 1
	}
	if height <= 0 {
		height = 1
	}
	m.width = width
	m.height = height

	navWidth := width / 3
	if navWidth < minNavWidth {
		navWidth = minNavWidth
	}
	if navWidth > maxNavWidth {
		navWidth = maxNavWidth
	}
	if navWidth > width-gutterWidth-1 {
		navWidth = width - gutterWidth - 1
	}
	m.navWidth = navWidth
	m.detailWidth = width - navWidth - gutterWidth

	if m.nav != nil {
		m.nav.SetSize(m.navWidth, m.height)
	}
	if m.detail != nil {
		m.detail.SetSize(m.detailWidth, m.height)
	}
	if m.add != nil {
		m.add.SetSize(m.width, m.height)
	}
}

// FocusNav gives focus to the navigation pane.
func (m *Model) FocusNav() tea.Cmd {
	if m.add != nil {
		return nil
	}
	m.focus = FocusNav
	var cmds []tea.Cmd
	if m.detail != nil {
		if cmd := m.detail.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.nav != nil {
		if cmd := m.nav.Focus(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// FocusDetail gives focus to the detail pane.
func (m *Model) FocusDetail() tea.Cmd {
	if m.add != nil {
		return nil
	}
	m.focus = FocusDetail
	var cmds []tea.Cmd
	if m.nav != nil {
		if cmd := m.nav.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.detail != nil {
		if cmd := m.detail.Focus(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// FocusedPane reports which pane currently owns focus.
func (m *Model) FocusedPane() FocusPane {
	return m.focus
}

// Update routes messages between the child panes and any active overlay.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	dbg := func(ctx, detail string) {
		if ctx == "" && detail == "" {
			return
		}
		cmds = appendCmd(cmds, events.DebugCmd(events.ComponentID("journal"), ctx, detail))
	}

	if m.add != nil {
		switch msg.(type) {
		case tea.WindowSizeMsg:
			m.add.SetSize(m.width, m.height)
		}
		if _, cmd := m.add.Update(msg); cmd != nil {
			cmds = appendCmd(cmds, cmd)
		}
	}

	keyMsg, isKey := msg.(tea.KeyMsg)
	blockKeys := m.add != nil && isKey

	if !blockKeys {
		if m.focus == FocusNav {
			if m.nav != nil {
				if _, cmd := m.nav.Update(msg); cmd != nil {
					cmds = appendCmd(cmds, cmd)
				}
			}
			if m.detail != nil {
				if _, cmd := m.detail.Update(msg); cmd != nil {
					cmds = appendCmd(cmds, cmd)
				}
			}
		} else {
			if m.detail != nil {
				if _, cmd := m.detail.Update(msg); cmd != nil {
					cmds = appendCmd(cmds, cmd)
				}
			}
			if m.nav != nil {
				if _, cmd := m.nav.Update(msg); cmd != nil {
					cmds = appendCmd(cmds, cmd)
				}
			}
		}
	}

	switch evt := msg.(type) {
	case events.BulletHighlightMsg:
		if m.add == nil && m.detailID != "" && evt.Component == m.detailID && m.nav != nil {
			ref := events.CollectionRef{ID: evt.Collection.ID, Name: evt.Collection.Title}
			dbg("bullet-highlight", fmt.Sprintf("select nav collection %s", ref.Label()))
			if cmd := m.nav.SelectCollection(ref); cmd != nil {
				cmds = appendCmd(cmds, cmd)
			}
		}
	case events.CollectionSelectMsg:
		if m.add == nil && evt.Component == m.navID {
			dbg("collection-select", fmt.Sprintf("focus detail for %s", evt.Collection.Label()))
			if cmd := m.FocusDetail(); cmd != nil {
				cmds = appendCmd(cmds, cmd)
			}
		}
	case events.BlurMsg:
		if m.add != nil && evt.Component == m.addID {
			dbg("add-overlay", "closing add overlay")
			if cmd := m.closeAddOverlay(); cmd != nil {
				cmds = appendCmd(cmds, cmd)
			}
		}
	}

	if m.add == nil && isKey && keyMsg.String() == "i" {
		if cmd := m.openAddForFocus(); cmd != nil {
			cmds = appendCmd(cmds, cmd)
		}
	}

	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

// View renders the nav/detail panes side by side, optionally overlaying the add-task dialog.
func (m *Model) View() (string, *tea.Cursor) {
	var navView, detailView string
	if m.nav != nil {
		navView = m.nav.View()
	}
	if m.detail != nil {
		detailView = m.detail.View()
	}
	navBlock := lipgloss.NewStyle().
		Width(m.navWidth).
		Height(m.height).
		Render(navView)
	detailBlock := lipgloss.NewStyle().
		Width(m.detailWidth).
		Height(m.height).
		Render(detailView)
	gutter := lipgloss.NewStyle().
		Width(gutterWidth).
		Height(m.height).
		Render(strings.Repeat(" ", gutterWidth))
	base := lipgloss.JoinHorizontal(
		lipgloss.Top,
		navBlock,
		gutter,
		detailBlock,
	)

	if m.add == nil {
		return base, nil
	}

	overlayView, cursor := m.add.View()
	width := m.width
	if width <= 0 {
		width = lipgloss.Width(base)
	}
	if width <= 0 {
		width = lipgloss.Width(overlayView)
	}
	height := m.height
	if height <= 0 {
		height = lipgloss.Height(base)
	}
	if height <= 0 {
		height = lipgloss.Height(overlayView)
	}

	overlay := lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Top,
		overlayView,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("238"))),
	)

	if cursor != nil {
		offsetX := 0
		viewWidth := lipgloss.Width(overlayView)
		if viewWidth < width {
			offsetX = (width - viewWidth) / 2
		}
		cursor = offsetCursor(cursor, offsetX, 0)
	}

	return overlay, cursor
}

func (m *Model) openAddForFocus() tea.Cmd {
	if m.cache == nil || m.add != nil {
		return nil
	}
	switch m.focus {
	case FocusDetail:
		if m.detail == nil {
			return nil
		}
		section, bullet, ok := m.detail.CurrentSelection()
		if !ok {
			return nil
		}
		collectionID := strings.TrimSpace(section.ID)
		if collectionID == "" {
			return nil
		}
		label := section.Title
		if strings.TrimSpace(label) == "" {
			label = collectionLabelFromID(collectionID)
		}
		opts := addtask.Options{
			InitialCollectionID:    collectionID,
			InitialCollectionLabel: label,
		}
		if strings.TrimSpace(bullet.ID) != "" {
			opts.InitialParentBulletID = strings.TrimSpace(bullet.ID)
		}
		return m.openAddOverlay(opts, FocusDetail)
	case FocusNav:
		if m.nav == nil {
			return nil
		}
		col, _, ok := m.nav.SelectedCollection()
		if !ok || col == nil {
			return nil
		}
		collectionID := strings.TrimSpace(col.ID)
		if collectionID == "" {
			collectionID = strings.TrimSpace(col.Name)
		}
		if collectionID == "" {
			return nil
		}
		label := col.Name
		if strings.TrimSpace(label) == "" {
			label = collectionLabelFromID(collectionID)
		}
		opts := addtask.Options{
			InitialCollectionID:    collectionID,
			InitialCollectionLabel: label,
		}
		return m.openAddOverlay(opts, FocusNav)
	default:
		return nil
	}
}

func (m *Model) openAddOverlay(opts addtask.Options, origin FocusPane) tea.Cmd {
	if m.cache == nil {
		return nil
	}
	addModel := addtask.NewModel(m.cache, opts)
	if m.addID != "" {
		addModel.SetID(m.addID)
	}
	addModel.SetSize(m.width, m.height)
	m.add = addModel
	m.addOrigin = origin

	var cmds []tea.Cmd
	switch origin {
	case FocusNav:
		if m.nav != nil {
			cmds = appendCmd(cmds, m.nav.Blur())
		}
	case FocusDetail:
		if m.detail != nil {
			cmds = appendCmd(cmds, m.detail.Blur())
		}
	}
	if init := m.add.Init(); init != nil {
		cmds = appendCmd(cmds, init)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *Model) closeAddOverlay() tea.Cmd {
	if m.add == nil {
		return nil
	}
	origin := m.addOrigin
	m.add = nil
	m.addOrigin = 0
	switch origin {
	case FocusNav, FocusDetail:
		return m.FocusDetail()
	default:
		return nil
	}
}

func appendCmd(cmds []tea.Cmd, cmd tea.Cmd) []tea.Cmd {
	if cmd == nil {
		return cmds
	}
	return append(cmds, cmd)
}

func offsetCursor(cursor *tea.Cursor, dx, dy int) *tea.Cursor {
	if cursor == nil {
		return nil
	}
	clone := *cursor
	clone.X += dx
	clone.Y += dy
	return &clone
}

func collectionLabelFromID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "(unnamed)"
	}
	parts := strings.Split(id, "/")
	return parts[len(parts)-1]
}
