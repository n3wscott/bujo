package bulletdetail

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
)

// Model renders a read-only view of a bullet's metadata.
type Model struct {
	title        string
	subtitle     string
	width        int
	height       int
	entry        *entry.Entry
	loading      bool
	err          error
	parentLabel  string
	collectionID string
}

// New constructs a detail model with the provided context labels.
func New(collectionTitle, bulletLabel, collectionID, parentLabel string) *Model {
	return &Model{
		title:        strings.TrimSpace(collectionTitle),
		subtitle:     strings.TrimSpace(bulletLabel),
		loading:      true,
		collectionID: strings.TrimSpace(collectionID),
		parentLabel:  strings.TrimSpace(parentLabel),
	}
}

// Init implements command.Overlay.
func (m *Model) Init() tea.Cmd { return nil }

// Update consumes messages for completeness; currently read-only.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.WindowSizeMsg:
		// size handled externally
	}
	return m, nil
}

// View renders the bullet detail panel.
func (m *Model) View() (string, *tea.Cursor) {
	header := lipgloss.NewStyle().Bold(true).Render(m.headerLine())
	body := ""
	switch {
	case m.loading:
		body = "Loading bullet…"
	case m.err != nil:
		body = lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Render("Error: " + m.err.Error())
	case m.entry != nil:
		body = m.renderEntry()
	default:
		body = "Bullet unavailable."
	}
	content := lipgloss.JoinVertical(lipgloss.Left, header, "", body)
	if m.width > 0 {
		content = lipgloss.NewStyle().Width(m.width).Render(content)
	}
	return content, nil
}

// SetSize configures the overlay dimensions.
func (m *Model) SetSize(width, height int) {
	if width < 40 {
		width = 40
	}
	if height < 10 {
		height = 10
	}
	m.width = width
	m.height = height
}

// SetLoading toggles the loading state.
func (m *Model) SetLoading(loading bool) {
	m.loading = loading
	if loading {
		m.err = nil
	}
}

// SetError records a loading error.
func (m *Model) SetError(err error) {
	m.loading = false
	m.err = err
}

// SetEntry assigns the entry to display.
func (m *Model) SetEntry(entry *entry.Entry) {
	m.entry = entry
	m.loading = false
	m.err = nil
}

func (m *Model) headerLine() string {
	title := m.title
	if title == "" {
		title = m.collectionID
		if title == "" {
			title = "Collection"
		}
	}
	sub := m.subtitle
	if sub == "" {
		sub = "(untitled bullet)"
	}
	return fmt.Sprintf("%s — %s", title, sub)
}

func (m *Model) renderEntry() string {
	if m.entry == nil {
		return ""
	}
	lines := []string{
		m.metadataLine("Collection", m.collectionID),
		m.metadataLine("Bullet", glyphLabel(m.entry.Bullet)),
		m.metadataLine("Signifier", glyphLabel(m.entry.Signifier)),
		m.metadataLine("Created", formatTime(m.entry.Created.Time)),
	}
	if m.parentLabel != "" {
		lines = append(lines, m.metadataLine("Parent", m.parentLabel))
	}
	if m.entry.Immutable {
		lines = append(lines, m.metadataLine("Mutable", "Locked"))
	}
	message := strings.TrimSpace(m.entry.Message)
	if message != "" {
		lines = append(lines, "", lipgloss.NewStyle().Bold(true).Render("Message"), message)
	}
	if note := strings.TrimSpace(m.entry.Collection); note != "" && note != m.collectionID {
		lines = append(lines, "", m.metadataLine("Stored In", note))
	}
	if len(m.entry.History) > 0 {
		lines = append(lines, "", lipgloss.NewStyle().Bold(true).Render("History"))
		for _, record := range m.entry.History {
			lines = append(lines, "  • "+describeHistory(record))
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) metadataLine(label, value string) string {
	if strings.TrimSpace(value) == "" {
		value = "(none)"
	}
	lhs := lipgloss.NewStyle().Bold(true).Render(label + ":")
	return fmt.Sprintf("%s %s", lhs, value)
}

func glyphLabel(g interface{}) string {
	switch v := g.(type) {
	case glyph.Bullet:
		return symbolOrName(v.Glyph())
	case glyph.Signifier:
		return symbolOrName(v.Glyph())
	default:
		return fmt.Sprint(v)
	}
}

func symbolOrName(g glyph.Glyph) string {
	sym := strings.TrimSpace(g.Symbol)
	if sym != "" {
		return fmt.Sprintf("%s (%s)", sym, g.Meaning)
	}
	return g.Meaning
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "(unknown)"
	}
	return t.Local().Format("2006-01-02 15:04")
}

func describeHistory(record entry.HistoryRecord) string {
	ts := formatTime(record.Timestamp.Time)
	switch record.Action {
	case entry.HistoryActionAdded:
		return fmt.Sprintf("%s — added to %s", ts, record.To)
	case entry.HistoryActionCompleted:
		return fmt.Sprintf("%s — completed", ts)
	case entry.HistoryActionMoved:
		return fmt.Sprintf("%s — moved from %s to %s", ts, record.From, record.To)
	case entry.HistoryActionStruck:
		return fmt.Sprintf("%s — marked irrelevant", ts)
	default:
		return fmt.Sprintf("%s — %s", ts, record.Action)
	}
}
