package command

import (
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/tui/events"
	overlaymgr "tableflip.dev/bujo/pkg/tui/ui/overlay"
)

// Overlay defines the interface command overlays must satisfy.
type Overlay interface {
	tea.Model
	tea.CursorModel
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
	keys    keyMap

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
		keys:            defaultKeyMap(),
		suggestionIndex: -1,
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

// InInputMode reports if the prompt is active.
func (m *Model) InInputMode() bool { return m.mode == ModeInput }

// Value returns the current prompt contents.
func (m *Model) Value() string {
	return m.prompt.Value()
}
