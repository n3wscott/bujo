package app

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/tui/components/command"
)

type newCollectionCreateMsg struct {
	Name string
}

type newCollectionCancelledMsg struct{}

type newCollectionOverlay struct {
	input   textinput.Model
	logger  io.Writer
	confirm bool
	pending string
	errMsg  string

	width  int
	height int
}

func newNewCollectionOverlay(logger io.Writer) *newCollectionOverlay {
	ti := textinput.New()
	ti.Placeholder = "Collection name"
	ti.CharLimit = 256
	ti.Prompt = "> "
	return &newCollectionOverlay{
		input:  ti,
		logger: logger,
	}
}

func (o *newCollectionOverlay) Init() tea.Cmd {
	return o.input.Focus()
}

func (o *newCollectionOverlay) Update(msg tea.Msg) (command.Overlay, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyMsg:
		switch v.String() {
		case "enter":
			name := strings.TrimSpace(o.input.Value())
			if name == "" {
				o.errMsg = "Collection name cannot be empty"
				return o, nil
			}
			if !o.confirm {
				o.confirm = true
				o.pending = name
				o.errMsg = ""
				return o, nil
			}
			if o.pending != name {
				o.confirm = false
				o.pending = ""
				o.errMsg = ""
				return o, nil
			}
			o.confirm = false
			o.pending = ""
			o.errMsg = ""
			o.input.Blur()
			o.input.SetValue("")
			return o, func() tea.Msg {
				return newCollectionCreateMsg{Name: name}
			}
		case "esc":
			o.confirm = false
			o.pending = ""
			o.errMsg = ""
			o.input.Blur()
			o.input.SetValue("")
			return o, func() tea.Msg { return newCollectionCancelledMsg{} }
		default:
			o.errMsg = ""
		}
	}
	model, cmd := o.input.Update(msg)
	o.input = model
	if o.confirm && strings.TrimSpace(o.input.Value()) != o.pending {
		o.confirm = false
		o.pending = ""
	}
	if _, ok := msg.(tea.KeyMsg); !ok {
		o.errMsg = ""
	}
	return o, cmd
}

func (o *newCollectionOverlay) View() (string, *tea.Cursor) {
	title := lipgloss.NewStyle().Bold(true).Render("Create New Collection")
	value := strings.TrimSpace(o.input.Value())
	info := "Enter a name and press Enter. Esc cancels."
	if value != "" {
		info = "Press Enter to continue, Esc to cancel."
	}
	if o.confirm && o.pending != "" {
		info = fmt.Sprintf("Press Enter again to create %q, Esc to edit.", o.pending)
	}
	messageStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	if trimmed := strings.TrimSpace(o.errMsg); trimmed != "" {
		info = trimmed
		messageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
	}
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		o.input.View(),
		messageStyle.Render(info),
	)
	content := lipgloss.NewStyle().
		Width(o.width).
		Height(o.height).
		Align(lipgloss.Top, lipgloss.Left).
		Padding(1, 2).
		Render(body)
	return content, nil
}

func (o *newCollectionOverlay) SetSize(width, height int) {
	if width <= 0 {
		width = 60
	}
	if height <= 0 {
		height = 7
	}
	o.width = width
	o.height = height
}

func (o *newCollectionOverlay) Focus() tea.Cmd {
	return o.input.Focus()
}

func (o *newCollectionOverlay) Blur() tea.Cmd {
	o.input.Blur()
	return nil
}
