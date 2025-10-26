package journal

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/components/collectionnav"
	"tableflip.dev/bujo/pkg/tui/events"
)

// FocusPane identifies which child currently owns keyboard focus.
type FocusPane int

const (
	FocusNav FocusPane = iota
	FocusDetail
)

const (
	minNavWidth = 24
	maxNavWidth = 36
	gutterWidth = 1
)

// Model composes the collection nav and detail panes side by side.
type Model struct {
	nav    *collectionnav.Model
	detail *collectiondetail.Model

	width       int
	height      int
	navWidth    int
	detailWidth int

	focus    FocusPane
	navID    events.ComponentID
	detailID events.ComponentID
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// NewModel builds a journal component from the provided children.
func NewModel(nav *collectionnav.Model, detail *collectiondetail.Model) *Model {
	m := &Model{
		nav:    nav,
		detail: detail,
		focus:  FocusNav,
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
}

// FocusNav gives focus to the navigation pane.
func (m *Model) FocusNav() tea.Cmd {
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

// Update routes messages to the focused child first (then the other child to
// keep state in sync) and aggregates resulting commands.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if m.focus == FocusNav {
		if m.nav != nil {
			if _, cmd := m.nav.Update(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if m.detail != nil {
			if _, cmd := m.detail.Update(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	} else {
		if m.detail != nil {
			if _, cmd := m.detail.Update(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if m.nav != nil {
			if _, cmd := m.nav.Update(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	switch evt := msg.(type) {
	case events.BulletHighlightMsg:
		if m.detailID != "" && evt.Component == m.detailID && m.nav != nil {
			ref := events.CollectionRef{ID: evt.Collection.ID, Name: evt.Collection.Title}
			if cmd := m.nav.SelectCollection(ref); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

// View renders the nav/detail panes side by side.
func (m *Model) View() string {
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
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		navBlock,
		gutter,
		detailBlock,
	)
}
