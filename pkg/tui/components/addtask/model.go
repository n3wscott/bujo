package addtask

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/tui/events"
)

var focusColor = lipgloss.Color("212")

type focusField int

const (
	fieldParentBullet focusField = iota
	fieldBulletType
	fieldSignifier
	fieldTaskInput
)

// Options control initial state for the add task view.
type Options struct {
	InitialCollectionID    string
	InitialCollectionLabel string
	InitialParentBulletID  string
}

type collectionOption struct {
	ID    string
	Label string
	Meta  collection.Meta
}

type parentOption struct {
	ID    string
	Label string
}

// Model renders an overlay for inserting new tasks/bullets.
type Model struct {
	cache   Cache
	id      events.ComponentID
	focused bool
	keys    keyMap

	initialCollectionID    string
	initialCollectionLabel string

	width      int
	height     int
	fieldWidth int

	focus focusField

	collectionOptions []collectionOption
	collectionIndex   int

	parentOptions []parentOption
	parentIndex   int

	bulletOptions    []glyph.Bullet
	bulletIndex      int
	signifierOptions []glyph.Signifier
	signifierIndex   int

	taskInput textinput.Model

	errorMsg      string
	confirmReset  bool
	lastSubmitted time.Time
}

// NewModel constructs the add-task overlay bound to the provided cache.
func NewModel(cache Cache, opts Options) *Model {
	tInput := textinput.New()
	tInput.Placeholder = "Describe the task…"
	tInput.Focus()
	tInput.Prompt = ""

	m := &Model{
		cache:                  cache,
		id:                     events.ComponentID("addtask"),
		focused:                true,
		keys:                   defaultKeyMap(),
		focus:                  fieldTaskInput,
		taskInput:              tInput,
		initialCollectionID:    strings.TrimSpace(opts.InitialCollectionID),
		initialCollectionLabel: strings.TrimSpace(opts.InitialCollectionLabel),
		bulletOptions: []glyph.Bullet{
			glyph.Task,
			glyph.Note,
			glyph.Event,
		},
		signifierOptions: []glyph.Signifier{
			glyph.None,
			glyph.Priority,
			glyph.Inspiration,
			glyph.Investigation,
		},
	}

	m.refreshCollections()
	if m.initialCollectionID != "" {
		m.selectCollectionByID(m.initialCollectionID)
	}
	m.refreshParentOptions()
	if opts.InitialParentBulletID != "" {
		m.selectParentByID(opts.InitialParentBulletID)
	}
	m.updatePrompt()
	m.updateInputFocus()
	return m
}

// SetID overrides the component identifier used in emitted events.
func (m *Model) SetID(id events.ComponentID) {
	if id == "" {
		return
	}
	m.id = id
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		events.FocusCmd(m.id),
		m.taskInput.Focus(),
	)
}

// Update processes Bubble Tea messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if m.width == 0 && m.height == 0 {
			m.SetSize(msg.Width, msg.Height)
		}
	case events.CollectionChangeMsg, events.CollectionOrderMsg, events.BulletChangeMsg:
		m.refreshCollections()
		m.refreshParentOptions()
	case tea.KeyMsg:
		if cmd := m.handleKey(msg); cmd != nil {
			return m, cmd
		}
	case events.FocusMsg:
		if msg.Component == m.id {
			m.focused = true
			if cmd := m.updateInputFocus(); cmd != nil {
				return m, cmd
			}
		}
	case events.BlurMsg:
		if msg.Component == m.id {
			m.focused = false
			if cmd := m.updateInputFocus(); cmd != nil {
				return m, cmd
			}
		}
	}

	var cmd tea.Cmd
	m.taskInput, cmd = m.taskInput.Update(msg)

	return m, cmd
}
