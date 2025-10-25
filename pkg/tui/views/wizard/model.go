package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/tui/theme"
	"tableflip.dev/bujo/pkg/tui/ui"
)

// Ensure Model satisfies the Component interface.
var _ ui.Component = (*Model)(nil)

// Step represents the active wizard stage.
type Step int

// Wizard steps.
const (
	StepParent Step = iota
	StepName
	StepType
	StepConfirm
)

// Model tracks the collection wizard overlay state.
type Model struct {
	Active        bool
	Step          Step
	Parent        string
	ParentOptions []string
	ParentIndex   int
	Name          string
	Type          collection.Type
	SuggestedType collection.Type
	TypeOptions   []collection.Type
	TypeIndex     int

	width          int
	height         int
	overlayReserve int
	nameInputView  string

	theme theme.Theme
}

// New constructs a wizard view model.
func New(th theme.Theme) *Model {
	return &Model{
		Type:          collection.TypeGeneric,
		SuggestedType: collection.TypeGeneric,
		theme:         th,
	}
}

// Init implements ui.Component.
func (m *Model) Init() tea.Cmd { return nil }

// Update implements ui.Component.
func (m *Model) Update(msg tea.Msg) (ui.Component, tea.Cmd) { return m, nil }

// SetSize stores the available viewport size.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// View renders the wizard overlay.
func (m *Model) View() string {
	if !m.Active {
		m.overlayReserve = 0
		return ""
	}
	width := m.width
	if width <= 0 {
		width = 80
	}
	height := m.height
	if height <= 0 {
		height = 24
	}

	lines := []string{m.theme.Modal.Title.Render("New Collection"), ""}
	bodyStyle := m.theme.Modal.Body

	switch m.Step {
	case StepParent:
		lines = append(lines, bodyStyle.Render("Parent"))
		for i, name := range m.ParentOptionsWithRoot() {
			marker := "  "
			if i == m.ParentIndex {
				marker = "→ "
			}
			lines = append(lines, bodyStyle.Render(marker+name))
		}
		lines = append(lines, "")
		lines = append(lines, bodyStyle.Render("↑/↓ move · Enter next · Esc cancel"))
	case StepName:
		lines = append(lines, bodyStyle.Render(fmt.Sprintf("Parent: %s", m.parentLabel())))
		lines = append(lines, "")
		lines = append(lines, bodyStyle.Render("Name"))
		lines = append(lines, bodyStyle.Render(m.nameInputView))
		lines = append(lines, "")
		lines = append(lines, bodyStyle.Render("Enter next · ctrl+b back · Esc cancel"))
	case StepType:
		lines = append(lines, bodyStyle.Render(fmt.Sprintf("Parent: %s", m.parentLabel())))
		if strings.TrimSpace(m.Name) != "" {
			lines = append(lines, bodyStyle.Render(fmt.Sprintf("Name: %s", m.Name)))
		}
		lines = append(lines, "")
		lines = append(lines, bodyStyle.Render("Type"))
		if len(m.TypeOptions) == 0 {
			m.TypeOptions = collection.AllTypes()
		}
		for i, typ := range m.TypeOptions {
			marker := "  "
			if i == m.TypeIndex {
				marker = "→ "
			}
			label := string(typ)
			desc := TypeDescription(typ)
			lines = append(lines, bodyStyle.Render(fmt.Sprintf("%s%s — %s", marker, label, desc)))
		}
		lines = append(lines, "")
		lines = append(lines, bodyStyle.Render("↑/↓ move · Enter next · ctrl+b back · Esc cancel"))
	case StepConfirm:
		path := m.Name
		if m.Parent != "" && m.Name != "" {
			path = JoinPath(m.Parent, m.Name)
		}
		display := truncateMiddle(path, m.modalDisplayLimit())
		lines = append(lines, bodyStyle.Render(fmt.Sprintf("Create %s as %s?", display, strings.ToLower(string(m.Type)))))
		lines = append(lines, "")
		lines = append(lines, bodyStyle.Render("Enter create · ctrl+b back · Esc cancel"))
	}

	content := strings.Join(lines, "\n")
	modalWidth := m.idealModalWidth(width)
	frame := m.theme.Modal.Frame.Copy().Width(modalWidth)
	panel := frame.Render(content)
	m.overlayReserve = strings.Count(panel, "\n") + 1
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
}

// OverlayReserve reports the number of lines consumed by the overlay.
func (m *Model) OverlayReserve() int {
	if !m.Active {
		return 0
	}
	return m.overlayReserve
}

// SetNameInputView updates the rendered input line for the name step.
func (m *Model) SetNameInputView(view string) {
	m.nameInputView = strings.TrimSuffix(view, "\n")
}

// ParentOptionsWithRoot returns the parent list with a synthetic root placeholder.
func (m *Model) ParentOptionsWithRoot() []string {
	options := make([]string, 1+len(m.ParentOptions))
	options[0] = "<root>"
	copy(options[1:], m.ParentOptions)
	return options
}

// JoinPath joins parent/name with a slash when both segments exist.
func JoinPath(parent, name string) string {
	parent = strings.TrimSpace(parent)
	name = strings.TrimSpace(name)
	if parent == "" {
		return name
	}
	if name == "" {
		return parent
	}
	return parent + "/" + name
}

// TypeDescription returns a friendly description for the requested collection type.
func TypeDescription(typ collection.Type) string {
	switch typ {
	case collection.TypeMonthly:
		return "Month folders (e.g., Future log)"
	case collection.TypeDaily:
		return "Daily pages rendered in calendar"
	case collection.TypeTracking:
		return "Numeric trackers grouped under Tracking"
	default:
		return "Generic free-form collection"
	}
}

func (m *Model) parentLabel() string {
	if strings.TrimSpace(m.Parent) == "" {
		return "<root>"
	}
	return m.Parent
}

func (m *Model) idealModalWidth(width int) int {
	modalWidth := width - 8
	if modalWidth > 60 {
		modalWidth = 60
	}
	if modalWidth < 24 {
		modalWidth = width - 4
		if modalWidth < 20 {
			modalWidth = 20
		}
	}
	return modalWidth
}

func (m *Model) modalDisplayLimit() int {
	max := m.width
	if max <= 0 {
		max = 80
	}
	limit := max - 18
	if limit < 10 {
		limit = 10
	}
	return limit
}

func truncateMiddle(s string, limit int) string {
	runes := []rune(s)
	if limit <= 0 || len(runes) <= limit {
		return s
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	left := (limit - 1) / 2
	right := limit - left - 1
	return string(runes[:left]) + "…" + string(runes[len(runes)-right:])
}
