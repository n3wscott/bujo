package command

import (
	"strings"

	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/tui/events"
	overlaymgr "tableflip.dev/bujo/pkg/tui/ui/overlay"
)

// Overlay defines the interface command overlays must satisfy.
type Overlay interface {
	Init() tea.Cmd
	Update(tea.Msg) (Overlay, tea.Cmd)
	View() (string, *tea.Cursor)
	SetSize(width, height int)
}

// OverlayPlacement controls where the overlay is rendered relative to the
// content viewport.
type OverlayPlacement struct {
	Width      int
	Height     int
	Horizontal lipgloss.Position
	Vertical   lipgloss.Position
	MarginX    int
	MarginY    int
	Fullscreen bool
}

// Options configures the command bar.
type Options struct {
	ID           events.ComponentID
	PromptPrefix string
	Placeholder  string
	StatusText   string
}

// Mode indicates the active behavior of the command bar.
type Mode int

const (
	ModePassive Mode = iota
	ModeInput
)

// Model renders a sticky command bar with optional overlay support.
type Model struct {
	id      events.ComponentID
	mode    Mode
	focused bool

	width         int
	height        int
	contentHeight int

	contentView   string
	contentCursor *tea.Cursor

	status string

	prompt       textinput.Model
	promptPrefix string

	lastPromptValue string

	overlay overlayState
}

type overlayState struct {
	model     Overlay
	placement OverlayPlacement
}

// NewModel constructs a command bar with the provided options.
func NewModel(opts Options) *Model {
	prompt := textinput.New()
	prompt.Placeholder = opts.Placeholder
	prompt.Prompt = ""
	prompt.Focus()
	prompt.Blur()

	id := opts.ID
	if id == "" {
		id = events.ComponentID("command")
	}

	return &Model{
		id:           id,
		mode:         ModePassive,
		status:       opts.StatusText,
		prompt:       prompt,
		promptPrefix: opts.PromptPrefix,
	}
}

// ID exposes the component identifier.
func (m *Model) ID() events.ComponentID { return m.id }

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// SetSize configures the viewport dimensions the component manages.
func (m *Model) SetSize(width, height int) {
	if width <= 0 {
		width = 1
	}
	if height <= 1 {
		height = 2
	}
	m.width = width
	m.height = height
	m.contentHeight = height - 1
	if m.contentHeight <= 0 {
		m.contentHeight = 1
	}
	promptWidth := width - len(m.promptPrefix)
	if promptWidth < 5 {
		promptWidth = width - 1
		if promptWidth < 1 {
			promptWidth = 1
		}
	}
	m.prompt.SetWidth(promptWidth)
	if m.overlay.model != nil {
		w, h := m.overlaySize()
		m.overlay.model.SetSize(w, h)
	}
}

// SetContent stores the content view that should appear above the command bar.
func (m *Model) SetContent(view string, cursor *tea.Cursor) {
	m.contentView = view
	if cursor != nil {
		copy := *cursor
		m.contentCursor = &copy
	} else {
		m.contentCursor = nil
	}
}

// SetStatus updates the passive status text.
func (m *Model) SetStatus(text string) {
	m.status = text
	if m.mode == ModePassive {
		m.lastPromptValue = ""
	}
}

// Focus ensures the command component receives focus.
func (m *Model) Focus() {
	m.focused = true
	if m.mode == ModeInput {
		m.prompt.Focus()
	}
}

// Blur releases focus.
func (m *Model) Blur() {
	m.focused = false
	m.prompt.Blur()
}

// BeginInput switches the command bar into input mode.
func (m *Model) BeginInput(initial string) tea.Cmd {
	m.mode = ModeInput
	m.prompt.SetValue(initial)
	m.lastPromptValue = initial
	m.prompt.CursorEnd()
	m.Focus()
	return tea.Batch(m.prompt.Focus(), events.CommandChangeCmd(m.id, initial, events.CommandModeInput))
}

// ExitInput returns the command bar to passive mode.
func (m *Model) ExitInput() tea.Cmd {
	m.mode = ModePassive
	m.prompt.Blur()
	m.lastPromptValue = ""
	return tea.Batch(events.CommandChangeCmd(m.id, "", events.CommandModePassive))
}

// InInputMode reports if the prompt is active.
func (m *Model) InInputMode() bool { return m.mode == ModeInput }

// SetOverlay activates an overlay with the provided placement.
func (m *Model) SetOverlay(overlay Overlay, placement OverlayPlacement) tea.Cmd {
	if overlay == nil {
		return nil
	}
	if m.overlay.model != nil {
		m.CloseOverlay()
	}
	m.overlay = overlayState{
		model:     overlay,
		placement: placement,
	}
	w, h := m.overlaySize()
	overlay.SetSize(w, h)
	return overlay.Init()
}

// CloseOverlay dismisses any active overlay.
func (m *Model) CloseOverlay() {
	m.overlay = overlayState{}
}

// HasOverlay reports whether an overlay is currently mounted.
func (m *Model) HasOverlay() bool {
	return m.overlay.model != nil
}

// Value returns the current prompt contents.
func (m *Model) Value() string {
	return m.prompt.Value()
}

func (m *Model) overlaySize() (int, int) {
	if m.overlay.model == nil {
		return 0, 0
	}
	if m.overlay.placement.Fullscreen {
		return m.width, m.contentHeight
	}
	w := m.overlay.placement.Width
	if w <= 0 || w > m.width {
		w = m.width
	}
	h := m.overlay.placement.Height
	if h <= 0 || h > m.contentHeight {
		h = m.contentHeight
	}
	return w, h
}

// Update routes messages to the command prompt and overlay.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	overlayHandled := false
	if m.overlay.model != nil {
		prev := m.overlay.model
		next, cmd := prev.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
			overlayHandled = true
		}
		if next == nil {
			m.CloseOverlay()
			overlayHandled = true
		} else {
			m.overlay.model = next
		}
	}

	_, isKey := msg.(tea.KeyMsg)
	handledKey := overlayHandled && isKey

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if handledKey {
			break
		}
		switch msg.String() {
		case "esc":
			if m.mode == ModeInput {
				cmds = append(cmds, m.ExitInput(), events.CommandCancelCmd(m.id))
				m.prompt.Blur()
			}
		case "enter":
			if m.mode == ModeInput {
				value := strings.TrimSpace(m.prompt.Value())
				if value != "" {
					cmds = append(cmds, events.CommandSubmitCmd(m.id, value))
					m.SetStatus(value)
				}
				cmds = append(cmds, m.ExitInput())
			}
		default:
			if m.mode == ModePassive && msg.String() == ":" {
				cmds = append(cmds, m.BeginInput(""))
				return m, tea.Batch(cmds...)
			}
		}
	}

	if !handledKey && m.mode == ModeInput {
		prev := m.prompt.Value()
		var cmd tea.Cmd
		m.prompt, cmd = m.prompt.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if newVal := m.prompt.Value(); newVal != prev {
			m.lastPromptValue = newVal
			cmds = append(cmds, events.CommandChangeCmd(m.id, newVal, events.CommandModeInput))
		}
	}

	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

// View renders the combined content, overlay, and command bar.
func (m *Model) View() (string, *tea.Cursor) {
	content := normalizeHeight(m.contentView, m.contentHeight)

	var contentCursor *tea.Cursor
	if m.contentCursor != nil {
		copy := *m.contentCursor
		contentCursor = &copy
	}

	if m.overlay.model != nil {
		overlayView, _ := m.overlay.model.View()
		content = m.placeOverlay(content, overlayView)
	}

	bar, barCursor := m.renderCommandBar()
	if barCursor != nil {
		contentCursor = barCursor
	}

	if content != "" {
		content = content + "\n" + bar
	} else {
		content = bar
	}

	return content, contentCursor
}

func (m *Model) renderCommandBar() (string, *tea.Cursor) {
	var line string
	var cursor *tea.Cursor
	switch m.mode {
	case ModeInput:
		inputView := m.prompt.View()
		line = m.promptPrefix + inputView
		if c := m.prompt.Cursor(); c != nil {
			copy := *c
			copy.Position.X += len(m.promptPrefix)
			copy.Position.Y += m.contentHeight
			cursor = &copy
		}
	default:
		status := m.status
		if status == "" {
			status = "Ready"
		}
		statusStyle := lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("214"))
		value := statusStyle.Render(status)
		available := m.width - len(m.promptPrefix)
		if available < 0 {
			available = 0
		}
		line = lipgloss.NewStyle().Width(available).Align(lipgloss.Right).Render(value)
		line = m.promptPrefix + line
	}

	line = padToWidth(line, m.width)
	return line, cursor
}

func normalizeHeight(body string, height int) string {
	lines := strings.Split(body, "\n")
	if len(lines) > height {
		lines = lines[len(lines)-height:]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func padToWidth(s string, width int) string {
	current := lipgloss.Width(s)
	if current >= width {
		return lipgloss.NewStyle().Width(width).Render(s)
	}
	padding := strings.Repeat(" ", width-current)
	return s + padding
}

func (m *Model) overlayOffsets(overlayWidth, overlayHeight int) (int, int) {
	placement := m.overlay.placement
	// Horizontal offset
	var offsetX int
	switch placement.Horizontal {
	case lipgloss.Left:
		offsetX = placement.MarginX
	case lipgloss.Right:
		offsetX = m.width - overlayWidth - placement.MarginX
	default:
		offsetX = (m.width - overlayWidth) / 2
	}
	if offsetX < 0 {
		offsetX = 0
	}
	if offsetX > m.width-overlayWidth {
		offsetX = m.width - overlayWidth
	}
	if offsetX < 0 {
		offsetX = 0
	}
	// Vertical offset
	var offsetY int
	switch placement.Vertical {
	case lipgloss.Top:
		offsetY = placement.MarginY
	case lipgloss.Bottom:
		offsetY = m.contentHeight - overlayHeight - placement.MarginY
	default:
		offsetY = (m.contentHeight - overlayHeight) / 2
	}
	if offsetY < 0 {
		offsetY = 0
	}
	if offsetY > m.contentHeight-overlayHeight {
		offsetY = m.contentHeight - overlayHeight
	}
	if offsetY < 0 {
		offsetY = 0
	}
	return offsetX, offsetY
}

func expandToWidth(s string, width int) []rune {
	runes := []rune(s)
	if len(runes) > width {
		return runes[:width]
	}
	if len(runes) < width {
		padding := make([]rune, width-len(runes))
		for i := range padding {
			padding[i] = ' '
		}
		runes = append(runes, padding...)
	}
	return runes
}
func (m *Model) placeOverlay(base string, overlay string) string {
	if overlay == "" {
		return base
	}
	placement := overlaymgr.Placement{
		Horizontal: m.overlay.placement.Horizontal,
		Vertical:   m.overlay.placement.Vertical,
		MarginX:    m.overlay.placement.MarginX,
		MarginY:    m.overlay.placement.MarginY,
		Width:      m.overlay.placement.Width,
		Height:     m.overlay.placement.Height,
	}
	if m.overlay.placement.Fullscreen {
		placement.Horizontal = lipgloss.Left
		placement.Vertical = lipgloss.Top
		placement.MarginX = 0
		placement.MarginY = 0
		placement.Width = m.width
		placement.Height = m.contentHeight
	}
	return overlaymgr.Compose(base, m.width, m.contentHeight, overlay, placement)
}
