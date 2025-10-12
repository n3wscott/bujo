package bottombar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/glyph"
)

// Mode represents the UI mode that influences footer layout.
type Mode int

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
	mode            Mode
	helpLine        string
	statusLine      string
	pendingBullet   glyph.Bullet
	commandInput    string
	commandView     string
	commandOptions  []CommandOption
	filteredOptions []CommandOption
	maxSuggestions  int
}

var (
	helpStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	statusStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	bulletStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	commandNameStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("212")).
				Bold(true)
	commandDescStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

// New returns a footer model with sensible defaults.
func New() Model {
	return Model{
		mode:           ModeNormal,
		pendingBullet:  glyph.Task,
		maxSuggestions: 6,
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
	m.commandView = ":" + view
	m.filterSuggestions(value)
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
		for i := 0; i < limit; i++ {
			opt := m.filteredOptions[i]
			name := commandNameStyle.Render(":" + opt.Name)
			desc := commandDescStyle.Render(opt.Description)
			if opt.Description == "" {
				lines = append(lines, name)
			} else {
				lines = append(lines, fmt.Sprintf("%s  %s", name, desc))
			}
		}
	}
	commandLine := m.commandView
	if commandLine == "" {
		commandLine = ":"
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
		return
	}
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
