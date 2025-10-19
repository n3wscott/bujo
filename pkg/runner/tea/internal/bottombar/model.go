// Package bottombar renders and manages the TUI footer component.
package bottombar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/glyph"
)

// Mode represents the UI mode that influences footer layout.
type Mode int

// Mode values define the bottom bar rendering state.
const (
	ModeNormal Mode = iota
	ModeInsert
	ModeCommand
	ModeHelp
	ModeBulletSelect
)

// CommandOption describes a command palette entry.
type CommandOption struct {
	Name        string
	Description string
}

// Model tracks footer/help/status rendering state.
type Model struct {
	mode             Mode
	helpLine         string
	statusLine       string
	pendingBullet    glyph.Bullet
	commandInput     string
	commandView      string
	commandOptions   []CommandOption
	filteredOptions  []CommandOption
	maxSuggestions   int
	suggestionIndex  int
	commandPrefix    string
	suggestionOffset int
}

var (
	helpStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	statusStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	bulletStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	commandNameStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("212")).
				Bold(true)
	commandDescStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	commandSelectedNameStyle = commandNameStyle.Reverse(true)
	commandSelectedDescStyle = commandDescStyle.Reverse(true)
)

// New returns a footer model with sensible defaults.
func New() Model {
	return Model{
		mode:             ModeNormal,
		pendingBullet:    glyph.Task,
		maxSuggestions:   6,
		suggestionIndex:  -1,
		commandPrefix:    ":",
		suggestionOffset: 0,
	}
}

// SetMode updates the visual mode.
func (m *Model) SetMode(mode Mode) {
	if m.mode == mode {
		return
	}
	m.mode = mode
	if mode != ModeCommand {
		m.filteredOptions = nil
		m.commandInput = ""
		m.commandView = ""
		m.suggestionIndex = -1
		m.suggestionOffset = 0
		m.commandPrefix = ":"
	}
}

// SetHelp sets the contextual help line.
func (m *Model) SetHelp(help string) {
	m.helpLine = help
}

// SetStatus sets the status message to display.
func (m *Model) SetStatus(status string) {
	m.statusLine = status
}

// SetPendingBullet updates the default bullet preview.
func (m *Model) SetPendingBullet(b glyph.Bullet) {
	m.pendingBullet = b
}

// SetCommandDefinitions configures the available command palette entries.
func (m *Model) SetCommandDefinitions(cmds []CommandOption) {
	m.commandOptions = cmds
	m.filterSuggestions(m.commandInput)
}

// UpdateCommandInput refreshes the command palette filter and rendered line.
func (m *Model) UpdateCommandInput(value string, view string) {
	m.commandInput = value
	m.commandView = view
	m.suggestionIndex = -1
	m.filterSuggestions(value)
}

// UpdateCommandPreview updates the rendered command line without refiltering suggestions.
func (m *Model) UpdateCommandPreview(value string, view string) {
	m.commandInput = value
	m.commandView = view
}

// Suggestion returns the filtered command option at index if available.
func (m Model) Suggestion(idx int) (CommandOption, bool) {
	if m.mode != ModeCommand {
		return CommandOption{}, false
	}
	if idx < 0 || idx >= len(m.filteredOptions) {
		return CommandOption{}, false
	}
	return m.filteredOptions[idx], true
}

// StepSuggestion advances the highlighted suggestion by delta, wrapping around.
func (m *Model) StepSuggestion(delta int) (CommandOption, bool) {
	if m.mode != ModeCommand || len(m.filteredOptions) == 0 || delta == 0 {
		return CommandOption{}, false
	}
	if m.suggestionIndex == -1 {
		if delta > 0 {
			m.suggestionIndex = 0
		} else {
			m.suggestionIndex = len(m.filteredOptions) - 1
		}
	} else {
		size := len(m.filteredOptions)
		m.suggestionIndex = (m.suggestionIndex + delta) % size
		if m.suggestionIndex < 0 {
			m.suggestionIndex += size
		}
	}
	m.updateSuggestionWindow()
	return m.filteredOptions[m.suggestionIndex], true
}

// ClearSuggestion removes any active suggestion highlight.
func (m *Model) ClearSuggestion() {
	m.suggestionIndex = -1
	m.updateSuggestionWindow()
}

// CurrentSuggestion returns the highlighted command if any.
func (m Model) CurrentSuggestion() (CommandOption, bool) {
	if m.mode != ModeCommand || m.suggestionIndex < 0 || m.suggestionIndex >= len(m.filteredOptions) {
		return CommandOption{}, false
	}
	return m.filteredOptions[m.suggestionIndex], true
}

// Height reports the number of lines consumed by the footer.
func (m Model) Height() int {
	switch m.mode {
	case ModeCommand:
		lines := len(m.filteredOptions)
		if lines > m.maxSuggestions {
			lines = m.maxSuggestions
		}
		// Include command input line.
		return lines + 1
	default:
		return 1
	}
}

// ExtraHeight returns lines beyond the baseline single footer row.
func (m Model) ExtraHeight() int {
	h := m.Height()
	if h <= 1 {
		return 0
	}
	return h - 1
}

// View renders the footer string and reports lines consumed.
func (m Model) View() (string, int) {
	switch m.mode {
	case ModeCommand:
		return m.renderCommandMode()
	default:
		return m.renderStatusLine(), 1
	}
}

func (m Model) renderStatusLine() string {
	var segments []string
	if m.helpLine != "" {
		segments = append(segments, helpStyle.Render(m.helpLine))
	}
	if m.statusLine != "" {
		segments = append(segments, statusStyle.Render(m.statusLine))
	}
	if m.pendingBullet != "" {
		bullet := fmt.Sprintf("bullet %s", m.pendingBullet.String())
		segments = append(segments, bulletStyle.Render(bullet))
	}
	if len(segments) == 0 {
		return " "
	}
	return strings.Join(segments, " │ ")
}

func (m Model) renderCommandMode() (string, int) {
	var lines []string
	if len(m.filteredOptions) == 0 && m.statusLine != "" {
		lines = append(lines, statusStyle.Render(m.statusLine))
	} else {
		limit := m.maxSuggestions
		if limit <= 0 {
			limit = len(m.filteredOptions)
		}
		if limit > len(m.filteredOptions) {
			limit = len(m.filteredOptions)
		}
		start := m.suggestionOffset
		if start < 0 {
			start = 0
		}
		if start > len(m.filteredOptions) {
			start = len(m.filteredOptions)
		}
		end := start + limit
		if end > len(m.filteredOptions) {
			end = len(m.filteredOptions)
		}
		for idx := start; idx < end; idx++ {
			opt := m.filteredOptions[idx]
			marker := "  "
			nameStyle := commandNameStyle
			descStyle := commandDescStyle
			if idx == m.suggestionIndex {
				marker = "→ "
				nameStyle = commandSelectedNameStyle
				descStyle = commandSelectedDescStyle
			}
			name := nameStyle.Render(opt.Name)
			if opt.Description == "" {
				lines = append(lines, marker+name)
				continue
			}
			desc := descStyle.Render(opt.Description)
			lines = append(lines, fmt.Sprintf("%s%s  %s", marker, name, desc))
		}
	}
	commandLine := m.commandView
	if commandLine == "" {
		commandLine = ""
	}
	if m.commandPrefix != "" {
		commandLine = m.commandPrefix + commandLine
	}
	lines = append(lines, commandLine)
	return strings.Join(lines, "\n"), len(lines)
}

func (m *Model) filterSuggestions(prefix string) {
	if m.mode != ModeCommand {
		m.filteredOptions = nil
		return
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		m.filteredOptions = append([]CommandOption(nil), m.commandOptions...)
	} else {
		if len(m.filteredOptions) > 0 {
			m.filteredOptions = m.filteredOptions[:0]
		} else {
			m.filteredOptions = make([]CommandOption, 0, len(m.commandOptions))
		}
		for _, opt := range m.commandOptions {
			if strings.HasPrefix(strings.ToLower(opt.Name), strings.ToLower(prefix)) {
				m.filteredOptions = append(m.filteredOptions, opt)
			}
		}
	}
	if len(m.filteredOptions) == 0 {
		m.suggestionIndex = -1
		m.suggestionOffset = 0
	} else {
		m.suggestionIndex = -1
		m.updateSuggestionWindow()
	}
}

// SetCommandPrefix overrides the command prompt prefix when rendering input.
func (m *Model) SetCommandPrefix(prefix string) {
	if strings.TrimSpace(prefix) == "" {
		prefix = ":"
	}
	m.commandPrefix = prefix
	m.updateSuggestionWindow()
}

func (m *Model) updateSuggestionWindow() {
	total := len(m.filteredOptions)
	if total == 0 {
		m.suggestionOffset = 0
		return
	}
	limit := m.maxSuggestions
	if limit <= 0 || limit > total {
		limit = total
	}
	if m.suggestionIndex >= total {
		m.suggestionIndex = total - 1
	}
	if m.suggestionIndex >= 0 {
		if m.suggestionIndex < m.suggestionOffset {
			m.suggestionOffset = m.suggestionIndex
		} else if m.suggestionIndex >= m.suggestionOffset+limit {
			m.suggestionOffset = m.suggestionIndex - limit + 1
		}
	} else {
		m.suggestionOffset = total - limit
	}
	if m.suggestionOffset < 0 {
		m.suggestionOffset = 0
	}
	maxOffset := total - limit
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.suggestionOffset > maxOffset {
		m.suggestionOffset = maxOffset
	}
}
