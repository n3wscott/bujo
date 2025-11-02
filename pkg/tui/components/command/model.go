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
// SuggestionOption represents a possible command the prompt can surface.
type SuggestionOption struct {
	Name        string
	Description string
}

// Mode identifies the command component operating state.
type Mode int

const (
	// ModePassive displays the command bar in status mode.
	ModePassive Mode = iota
	// ModeInput places the command bar in interactive input mode.
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

	suggestions           []SuggestionOption
	filteredSuggestions   []SuggestionOption
	suggestionLimit       int
	suggestionIndex       int
	suggestionOriginal    string
	suggestionOverlay     string
	suggestionWindowStart int
	suggestionPlacement   overlaymgr.Placement
}

const overlayAlignLeft = lipgloss.Position(-1)

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
		id:              id,
		mode:            ModePassive,
		status:          opts.StatusText,
		prompt:          prompt,
		promptPrefix:    opts.PromptPrefix,
		suggestionLimit: 8,
		suggestionPlacement: overlaymgr.Placement{
			Horizontal: overlayAlignLeft,
			Vertical:   lipgloss.Bottom,
		},
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
	m.suggestionWindowStart = 0
	m.suggestionIndex = -1
	m.updateSuggestionWindow()
	m.refreshSuggestionOverlay()
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

// SetSuggestions configures the available suggestion list.
func (m *Model) SetSuggestions(options []SuggestionOption) {
	m.suggestions = append([]SuggestionOption(nil), options...)
	m.applySuggestionFilter(m.prompt.Value(), true)
}

// SetSuggestionLimit adjusts the maximum number of suggestions displayed.
func (m *Model) SetSuggestionLimit(limit int) {
	if limit <= 0 {
		limit = 8
	}
	m.suggestionLimit = limit
	m.updateSuggestionWindow()
	m.refreshSuggestionOverlay()
}

func (m *Model) applySuggestionFilter(value string, resetSelection bool) {
	if m.mode != ModeInput {
		m.filteredSuggestions = nil
		m.suggestionOverlay = ""
		m.suggestionWindowStart = 0
		return
	}

	prefix := strings.TrimSpace(strings.ToLower(value))
	matches := make([]SuggestionOption, 0, len(m.suggestions))
	if prefix == "" {
		matches = append(matches, m.suggestions...)
	} else {
		seen := make(map[string]struct{}, len(m.suggestions))
		for _, opt := range m.suggestions {
			name := strings.ToLower(opt.Name)
			if strings.HasPrefix(name, prefix) {
				matches = append(matches, opt)
				seen[opt.Name] = struct{}{}
			}
		}
		for _, opt := range m.suggestions {
			if _, ok := seen[opt.Name]; ok {
				continue
			}
			name := strings.ToLower(opt.Name)
			if strings.Contains(name, prefix) {
				matches = append(matches, opt)
			}
		}
	}

	if cap(m.filteredSuggestions) < len(matches) {
		m.filteredSuggestions = make([]SuggestionOption, 0, len(matches))
	}
	m.filteredSuggestions = m.filteredSuggestions[:0]
	m.filteredSuggestions = append(m.filteredSuggestions, matches...)

	if resetSelection {
		m.suggestionIndex = -1
		m.suggestionWindowStart = 0
		m.suggestionOriginal = value
	} else {
		if m.suggestionIndex >= len(m.filteredSuggestions) {
			m.suggestionIndex = len(m.filteredSuggestions) - 1
		}
		if m.suggestionIndex < -1 {
			m.suggestionIndex = -1
		}
	}

	m.updateSuggestionWindow()
	m.refreshSuggestionOverlay()
}

func (m *Model) effectiveSuggestionLimit() int {
	total := len(m.filteredSuggestions)
	if total == 0 {
		return 0
	}
	limit := m.suggestionLimit
	if limit <= 0 || limit > total {
		limit = total
	}
	maxRows := m.height - 1
	if maxRows < 0 {
		maxRows = 0
	}
	if limit > maxRows {
		limit = maxRows
	}
	if limit < 0 {
		limit = 0
	}
	return limit
}

func (m *Model) updateSuggestionWindow() {
	total := len(m.filteredSuggestions)
	if total == 0 {
		m.suggestionWindowStart = 0
		return
	}

	limit := m.effectiveSuggestionLimit()
	if limit <= 0 {
		m.suggestionWindowStart = 0
		return
	}

	if m.suggestionWindowStart > total-limit {
		m.suggestionWindowStart = total - limit
	}
	if m.suggestionWindowStart < 0 {
		m.suggestionWindowStart = 0
	}

	if m.suggestionIndex >= 0 {
		if m.suggestionIndex < m.suggestionWindowStart {
			m.suggestionWindowStart = m.suggestionIndex
		} else if m.suggestionIndex >= m.suggestionWindowStart+limit {
			m.suggestionWindowStart = m.suggestionIndex - limit + 1
		}
	}
}

func (m *Model) refreshSuggestionOverlay() {
	if m.mode != ModeInput || len(m.filteredSuggestions) == 0 {
		m.suggestionOverlay = ""
		return
	}
	limit := m.effectiveSuggestionLimit()
	if limit <= 0 {
		m.suggestionOverlay = ""
		return
	}
	start := m.suggestionWindowStart
	if start < 0 {
		start = 0
	}
	maxStart := len(m.filteredSuggestions) - limit
	if maxStart < 0 {
		maxStart = 0
	}
	if start > maxStart {
		start = maxStart
	}
	end := start + limit
	if end > len(m.filteredSuggestions) {
		end = len(m.filteredSuggestions)
	}

	maxWidth := 0
	count := end - start
	if count <= 0 {
		m.suggestionOverlay = ""
		return
	}
	rows := make([]string, count)
	nameStyle := lipgloss.NewStyle().Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	primaryStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	primaryDesc := lipgloss.NewStyle().Foreground(lipgloss.Color("213"))

	for i := start; i < end; i++ {
		opt := m.filteredSuggestions[i]
		marker := "  "
		nameRender := nameStyle.Render(opt.Name)
		descRender := descStyle.Render(strings.TrimSpace(opt.Description))
		if i == m.suggestionIndex {
			marker = "→ "
			nameRender = primaryStyle.Render(opt.Name)
			descRender = primaryDesc.Render(strings.TrimSpace(opt.Description))
		}
		line := marker + strings.TrimSpace(nameRender)
		if dr := strings.TrimSpace(descRender); dr != "" {
			line += "  " + dr
		}
		rowIdx := i - start
		rows[rowIdx] = line
		if w := lipgloss.Width(line); w > maxWidth {
			maxWidth = w
		}
	}

	availableWidth := m.width
	if availableWidth <= 0 {
		availableWidth = 10
	}
	if maxWidth > availableWidth {
		maxWidth = availableWidth
	}
	if maxWidth <= 0 {
		maxWidth = availableWidth
	}

	contentStyle := lipgloss.NewStyle().Width(maxWidth).Align(lipgloss.Left)
	for i := range rows {
		rows[i] = contentStyle.Render(rows[i])
	}

	m.suggestionOverlay = strings.Join(rows, "\n")
	height := strings.Count(m.suggestionOverlay, "\n") + 1
	if height <= 0 {
		height = len(rows)
		if height <= 0 {
			height = 1
		}
	}
	placementWidth := maxWidth
	if placementWidth <= 0 || placementWidth > m.width {
		placementWidth = m.width
	}
	m.suggestionPlacement = overlaymgr.Placement{
		Horizontal: overlayAlignLeft,
		Vertical:   lipgloss.Bottom,
		MarginX:    0,
		MarginY:    0,
		Width:      placementWidth,
		Height:     height,
	}
}

func (m *Model) cycleSuggestion(delta int) bool {
	if m.mode != ModeInput {
		return false
	}
	total := len(m.filteredSuggestions)
	if total == 0 {
		return false
	}
	if m.effectiveSuggestionLimit() == 0 {
		return false
	}
	if m.suggestionIndex == -1 {
		if delta > 0 {
			m.suggestionIndex = 0
		} else {
			m.suggestionIndex = total - 1
		}
		m.suggestionOriginal = m.prompt.Value()
	} else {
		m.suggestionIndex = (m.suggestionIndex + delta) % total
		if m.suggestionIndex < 0 {
			m.suggestionIndex += total
		}
	}
	if m.suggestionIndex < 0 || m.suggestionIndex >= total {
		m.clearSuggestionSelection()
		return false
	}
	choice := m.filteredSuggestions[m.suggestionIndex]
	m.prompt.SetValue(choice.Name)
	m.prompt.CursorEnd()
	m.updateSuggestionWindow()
	m.refreshSuggestionOverlay()
	return true
}

func (m *Model) clearSuggestionSelection() bool {
	if m.suggestionIndex == -1 {
		return false
	}
	m.prompt.SetValue(m.suggestionOriginal)
	m.prompt.CursorEnd()
	m.suggestionIndex = -1
	m.updateSuggestionWindow()
	m.refreshSuggestionOverlay()
	return true
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
	m.applySuggestionFilter(initial, true)
	return tea.Batch(m.prompt.Focus(), events.CommandChangeCmd(m.id, initial, events.CommandModeInput))
}

// ExitInput returns the command bar to passive mode.
func (m *Model) ExitInput() tea.Cmd {
	m.mode = ModePassive
	m.prompt.Blur()
	m.lastPromptValue = ""
	m.filteredSuggestions = nil
	m.suggestionOverlay = ""
	m.suggestionIndex = -1
	m.suggestionOriginal = ""
	m.suggestionWindowStart = 0
	return tea.Batch(events.CommandChangeCmd(m.id, "", events.CommandModePassive))
}

// InInputMode reports if the prompt is active.
func (m *Model) InInputMode() bool { return m.mode == ModeInput }

// Value returns the current prompt contents.
func (m *Model) Value() string {
	return m.prompt.Value()
}

// Update routes messages to the command prompt and suggestion overlay.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	handledKey := false

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if handledKey {
			break
		}
		switch msg.String() {
		case "esc":
			if m.mode == ModeInput {
				if m.clearSuggestionSelection() {
					handledKey = true
					newVal := m.prompt.Value()
					if newVal != m.lastPromptValue {
						m.lastPromptValue = newVal
						cmds = append(cmds, events.CommandChangeCmd(m.id, newVal, events.CommandModeInput))
					}
					break
				}
				handledKey = true
				cmds = append(cmds, m.ExitInput(), events.CommandCancelCmd(m.id))
				m.prompt.Blur()
			}
		case "enter":
			if m.mode == ModeInput {
				handledKey = true
				value := strings.TrimSpace(m.prompt.Value())
				if value != "" {
					cmds = append(cmds, events.CommandSubmitCmd(m.id, value))
					m.SetStatus(value)
				}
				cmds = append(cmds, m.ExitInput())
			}
		case "up", "shift+tab":
			if m.mode == ModeInput && m.cycleSuggestion(-1) {
				handledKey = true
				newVal := m.prompt.Value()
				if newVal != m.lastPromptValue {
					m.lastPromptValue = newVal
					cmds = append(cmds, events.CommandChangeCmd(m.id, newVal, events.CommandModeInput))
				}
			}
		case "down", "tab":
			if m.mode == ModeInput && m.cycleSuggestion(1) {
				handledKey = true
				newVal := m.prompt.Value()
				if newVal != m.lastPromptValue {
					m.lastPromptValue = newVal
					cmds = append(cmds, events.CommandChangeCmd(m.id, newVal, events.CommandModeInput))
				}
			}
		default:
			if m.mode == ModePassive && msg.String() == ":" {
				handledKey = true
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
			m.applySuggestionFilter(newVal, true)
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

	if m.suggestionOverlay != "" {
		content = overlaymgr.Compose(content, m.width, m.contentHeight, m.suggestionOverlay, m.suggestionPlacement)
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
			copy.X += len(m.promptPrefix)
			copy.Y = m.contentHeight
			cursor = &copy
		}
	default:
		status := m.status
		if status == "" {
			status = "Ready"
		}
		statusStyle := lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("214"))
		value := statusStyle.Render(status)
		available := m.width
		if available < 0 {
			available = 0
		}
		line = lipgloss.NewStyle().Width(available).Align(lipgloss.Right).Render(value)
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
