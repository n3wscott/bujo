package app

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/v2/textinput"
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

	creatingNew   bool
	createInput   textinput.Model
	createConfirm bool
	createPending string
	createError   string
}

type moveCreateCollectionMsg struct {
	Name string
}

type moveCreateCollectionCancelledMsg struct{}

func newMovebulletOverlay(detail *bulletdetail.Model, nav *collectionnav.Model, navOnRight bool, logger io.Writer) *movebulletOverlay {
	input := textinput.New()
	input.Placeholder = "New collection name"
	input.CharLimit = 256
	input.Prompt = "> "
	return &movebulletOverlay{
		detail:      detail,
		nav:         nav,
		navOnRight:  navOnRight,
		logger:      logger,
		createInput: input,
	}
}

func (o *movebulletOverlay) Init() tea.Cmd {
	if o.nav == nil {
		return nil
	}
	return o.nav.Focus()
}

func (o *movebulletOverlay) Update(msg tea.Msg) (command.Overlay, tea.Cmd) {
	if o.creatingNew {
		return o.updateCreateNewCollection(msg)
	}
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

	promptBlock := ""
	if o.creatingNew {
		info := "Enter a new collection name. Esc cancels."
		value := strings.TrimSpace(o.createInput.Value())
		if value != "" {
			info = "Press Enter to continue, Esc to cancel."
		}
		if o.createConfirm && o.createPending != "" {
			info = fmt.Sprintf("Press Enter again to create %q, Esc to edit.", o.createPending)
		}
		messageStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		text := info
		if trimmed := strings.TrimSpace(o.createError); trimmed != "" {
			messageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
			text = trimmed
		}
		message := messageStyle.Render(text)
		prompt := lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render("Create New Collection"),
			o.createInput.View(),
			message,
		)
		promptBlock = lipgloss.NewStyle().
			Width(o.width).
			Align(lipgloss.Left, lipgloss.Top).
			Render(prompt)
	}

	content := ""
	if o.navOnRight {
		content = lipgloss.JoinHorizontal(lipgloss.Top, detailBlock, divider, navBlock)
	} else {
		content = lipgloss.JoinHorizontal(lipgloss.Top, navBlock, divider, detailBlock)
	}
	content = lipgloss.NewStyle().Width(o.width).Align(lipgloss.Left, lipgloss.Top).Render(content)

	parts := []string{instructions}
	if promptBlock != "" {
		parts = append(parts, promptBlock)
	}
	parts = append(parts, content)

	body := lipgloss.JoinVertical(lipgloss.Left, parts...)
	frame := lipgloss.NewStyle().
		Width(o.width).
		Height(o.height).
		AlignVertical(lipgloss.Top)

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

func (o *movebulletOverlay) updateCreateNewCollection(msg tea.Msg) (command.Overlay, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyMsg:
		switch v.String() {
		case "enter":
			name := strings.TrimSpace(o.createInput.Value())
			if name == "" {
				o.createError = "Collection name cannot be empty"
				return o, nil
			}
			if !o.createConfirm {
				o.createConfirm = true
				o.createPending = name
				o.createError = ""
				return o, nil
			}
			if o.createPending != name {
				o.createConfirm = false
				o.createPending = ""
				o.createError = ""
				return o, nil
			}
			finalName := name
			o.creatingNew = false
			o.createConfirm = false
			o.createPending = ""
			o.createError = ""
			o.createInput.Blur()
			o.createInput.SetValue("")
			o.SetSize(o.width, o.height)
			return o, func() tea.Msg {
				return moveCreateCollectionMsg{Name: finalName}
			}
		case "esc":
			if o.createConfirm {
				o.createConfirm = false
				o.createPending = ""
				o.createError = ""
				return o, nil
			}
			o.creatingNew = false
			o.createConfirm = false
			o.createPending = ""
			o.createError = ""
			o.createInput.Blur()
			o.createInput.SetValue("")
			o.SetSize(o.width, o.height)
			return o, func() tea.Msg { return moveCreateCollectionCancelledMsg{} }
		default:
			o.createError = ""
		}
	}
	model, cmd := o.createInput.Update(msg)
	o.createInput = model
	if o.createConfirm && strings.TrimSpace(o.createInput.Value()) != o.createPending {
		o.createConfirm = false
		o.createPending = ""
	}
	if _, ok := msg.(tea.KeyMsg); !ok {
		o.createError = ""
	}
	return o, cmd
}

func (o *movebulletOverlay) BeginNewCollectionPrompt() tea.Cmd {
	o.creatingNew = true
	o.createConfirm = false
	o.createPending = ""
	o.createError = ""
	o.createInput.SetValue("")
	o.SetSize(o.width, o.height)
	var cmds []tea.Cmd
	if o.nav != nil {
		if cmd := o.nav.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if cmd := o.createInput.Focus(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (o *movebulletOverlay) FocusNav() tea.Cmd {
	if o.nav == nil {
		return nil
	}
	if o.creatingNew {
		o.creatingNew = false
		o.createConfirm = false
		o.createPending = ""
		o.createError = ""
		o.createInput.Blur()
		o.createInput.SetValue("")
		o.SetSize(o.width, o.height)
	}
	return o.nav.Focus()
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
	if o.creatingNew {
		height -= o.promptHeight()
	}
	if height <= 0 {
		height = 1
	}
	return height
}

func (o *movebulletOverlay) promptHeight() int {
	return 3
}
