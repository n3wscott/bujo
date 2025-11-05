package journal

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	cachepkg "tableflip.dev/bujo/pkg/tui/cache"
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

type addParentMode int

const (
	parentModeNone addParentMode = iota
	parentModeSelected
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
	id       events.ComponentID
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
		id:     events.ComponentID("journal"),
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

// SetID overrides the component identifier used in emitted events.
func (m *Model) SetID(id events.ComponentID) {
	if id == "" {
		return
	}
	m.id = id
}

// ID exposes the component identifier.
func (m *Model) ID() events.ComponentID {
	return m.id
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

// Update routes messages between the child panes and any active overlay.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	dbg := func(ctx, detail string) {
		if ctx == "" && detail == "" {
			return
		}
		cmds = appendCmd(cmds, events.DebugCmd(m.id, ctx, detail))
	}

	keyMsg, isKey := msg.(tea.KeyMsg)
	blockKeys := false
	if isKey {
		switch keyMsg.String() {
		case "tab":
			if m.focus == FocusNav {
				if cmd := m.FocusDetail(); cmd != nil {
					cmds = appendCmd(cmds, cmd)
				}
				blockKeys = true
			}
		case "shift+tab":
			if m.focus == FocusDetail {
				if cmd := m.FocusNav(); cmd != nil {
					cmds = appendCmd(cmds, cmd)
				}
				blockKeys = true
			}
		case "i":
			if cmd := m.requestAddForFocus(parentModeNone); cmd != nil {
				cmds = appendCmd(cmds, cmd)
			}
			blockKeys = true
		case "shift+i", "I":
			if cmd := m.requestAddForFocus(parentModeSelected); cmd != nil {
				cmds = appendCmd(cmds, cmd)
			}
			blockKeys = true
		}
	}

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
		if m.detailID != "" && evt.Component == m.detailID && m.nav != nil {
			ref := events.CollectionRef{ID: evt.Collection.ID, Name: evt.Collection.Title}
			dbg("bullet-highlight", fmt.Sprintf("select nav collection %s", ref.Label()))
			if cmd := m.nav.SelectCollection(ref); cmd != nil {
				cmds = appendCmd(cmds, cmd)
			}
		}
	case events.BulletSelectMsg:
		if evt.Component == m.detailID {
			if cmd := events.BulletDetailRequestCmd(m.id, evt.Collection, evt.Bullet); cmd != nil {
				cmds = appendCmd(cmds, cmd)
			}
		}
	case events.CollectionSelectMsg:
		if evt.Component == m.navID {
			dbg("collection-select", fmt.Sprintf("focus detail for %s", evt.Collection.Label()))
			if cmd := m.FocusDetail(); cmd != nil {
				cmds = appendCmd(cmds, cmd)
			}
		}
	}

	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

// View renders the nav/detail panes side by side.
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

	return base, nil
}

func (m *Model) requestAddForFocus(mode addParentMode) tea.Cmd {
	var collectionID, collectionLabel, parentID, parentLabel, origin string
	switch m.focus {
	case FocusDetail:
		if m.detail == nil {
			return nil
		}
		section, bullet, selParentID, selParentLabel, ok := m.detail.CurrentSelectionWithParent()
		if !ok {
			return nil
		}
		collectionID = strings.TrimSpace(section.ID)
		if collectionID == "" {
			return nil
		}
		collectionLabel = section.Title
		if strings.TrimSpace(collectionLabel) == "" {
			collectionLabel = collectionLabelFromID(collectionID)
		}
		if mode == parentModeSelected && strings.TrimSpace(bullet.ID) != "" {
			if trimmed := strings.TrimSpace(selParentID); trimmed != "" {
				parentID = trimmed
				parentLabel = strings.TrimSpace(selParentLabel)
				if parentLabel == "" {
					parentLabel = parentID
				}
			} else {
				parentID = strings.TrimSpace(bullet.ID)
				parentLabel = strings.TrimSpace(bullet.Label)
				if parentLabel == "" {
					parentLabel = parentID
				}
			}
		}
		origin = "detail"
	case FocusNav:
		if m.nav == nil {
			return nil
		}
		col, _, ok := m.nav.SelectedCollection()
		if !ok || col == nil {
			return nil
		}
		collectionID = strings.TrimSpace(col.ID)
		if collectionID == "" {
			collectionID = strings.TrimSpace(col.Name)
		}
		if collectionID == "" {
			return nil
		}
		collectionLabel = col.Name
		if strings.TrimSpace(collectionLabel) == "" {
			collectionLabel = collectionLabelFromID(collectionID)
		}
		origin = "nav"
	default:
		return nil
	}
	if collectionID == "" {
		return nil
	}
	return events.AddTaskRequestCmd(m.id, collectionID, collectionLabel, parentID, parentLabel, origin)
}

// CurrentSelection exposes the active detail section and highlighted bullet, if any.
func (m *Model) CurrentSelection() (collectiondetail.Section, collectiondetail.Bullet, bool) {
	if m.detail == nil {
		return collectiondetail.Section{}, collectiondetail.Bullet{}, false
	}
	return m.detail.CurrentSelection()
}

func appendCmd(cmds []tea.Cmd, cmd tea.Cmd) []tea.Cmd {
	if cmd == nil {
		return cmds
	}
	return append(cmds, cmd)
}

func collectionLabelFromID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "(unnamed)"
	}
	parts := strings.Split(id, "/")
	return parts[len(parts)-1]
}
