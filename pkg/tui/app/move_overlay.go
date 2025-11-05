package app

import (
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/tui/components/bulletdetail"
	collectionnav "tableflip.dev/bujo/pkg/tui/components/collectionnav"
	"tableflip.dev/bujo/pkg/tui/components/command"
)

type movebulletOverlay struct {
	detail      *bulletdetail.Model
	nav         *collectionnav.Model
	width       int
	height      int
	detailWidth int
	navWidth    int
	navOnRight  bool
	logger      io.Writer
}

func newMovebulletOverlay(detail *bulletdetail.Model, nav *collectionnav.Model, navOnRight bool, logger io.Writer) *movebulletOverlay {
	return &movebulletOverlay{detail: detail, nav: nav, navOnRight: navOnRight, logger: logger}
}

func (o *movebulletOverlay) Init() tea.Cmd {
	if o.nav == nil {
		return nil
	}
	return o.nav.Focus()
}

func (o *movebulletOverlay) Update(msg tea.Msg) (command.Overlay, tea.Cmd) {
	var cmds []tea.Cmd
	if o.nav != nil {
		next, cmd := o.nav.Update(msg)
		if nav, ok := next.(*collectionnav.Model); ok {
			o.nav = nav
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return o, nil
	}
	return o, tea.Batch(cmds...)
}

func (o *movebulletOverlay) View() (string, *tea.Cursor) {
	var detailView string
	if o.detail != nil {
		dv, _ := o.detail.View()
		detailView = dv
	}
	contentHeight := o.contentHeight()
	navView := ""
	if o.nav != nil {
		navView = o.nav.View()
	}

	navLinesSlice := strings.Split(navView, "\n")
	for len(navLinesSlice) > contentHeight && strings.TrimSpace(navLinesSlice[len(navLinesSlice)-1]) == "" {
		navLinesSlice = navLinesSlice[:len(navLinesSlice)-1]
	}
	navView = strings.Join(navLinesSlice, "\n")

	detailBlock := lipgloss.NewStyle().
		Width(o.detailWidth).
		Height(contentHeight).
		AlignVertical(lipgloss.Top).
		Render(detailView)
	navBlock := lipgloss.NewStyle().
		Width(o.navWidth).
		Height(contentHeight).
		AlignVertical(lipgloss.Top).
		Render(navView)

	divider := verticalDivider(contentHeight)

	instructions := lipgloss.NewStyle().
		Bold(true).
		Width(o.width).
		Render("Select destination and press Enter · Esc cancels")

	frame := lipgloss.NewStyle().
		Width(o.width).
		Height(o.height).
		AlignVertical(lipgloss.Top)

	if o.navOnRight {
		content := lipgloss.JoinHorizontal(lipgloss.Top, detailBlock, divider, navBlock)
		body := instructions + "\n" + content
		return frame.Render(body), nil
	}

	content := lipgloss.JoinHorizontal(lipgloss.Top, navBlock, divider, detailBlock)
	body := instructions + "\n" + content
	return frame.Render(body), nil
}

func (o *movebulletOverlay) SetSize(width, height int) {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 20
	}
	o.width = width
	o.height = height
	contentHeight := o.contentHeight()

	const (
		preferredNavWidth = 24
		minNavWidth       = 12
		minDetailWidth    = 40
	)

	maxNavAllowed := width - minDetailWidth - 1
	if maxNavAllowed < 0 {
		maxNavAllowed = 0
	}
	navWidth := maxNavAllowed
	if navWidth > preferredNavWidth {
		navWidth = preferredNavWidth
	}
	if maxNavAllowed >= minNavWidth && navWidth < minNavWidth {
		navWidth = minNavWidth
	}
	detailWidth := width - navWidth - 1
	if detailWidth < minDetailWidth {
		detailWidth = maxInt(0, width-navWidth-1)
	}

	o.detailWidth = detailWidth
	o.navWidth = navWidth

	if o.detail != nil {
		o.detail.SetSize(detailWidth, contentHeight)
	}
	if o.nav != nil {
		o.nav.SetSize(navWidth, contentHeight)
	}
}

func (o *movebulletOverlay) Focus() tea.Cmd {
	if o.nav != nil {
		return o.nav.Focus()
	}
	return nil
}

func (o *movebulletOverlay) Blur() tea.Cmd {
	if o.nav != nil {
		return o.nav.Blur()
	}
	return nil
}

func verticalDivider(height int) string {
	if height <= 0 {
		height = 1
	}
	lines := strings.Repeat("│\n", height-1) + "│"
	return lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(lines)
}

func (o *movebulletOverlay) contentHeight() int {
	height := o.height - 1
	if height <= 0 {
		height = 1
	}
	return height
}
