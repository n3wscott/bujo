package migration

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/tui/components/index"
	"tableflip.dev/bujo/pkg/tui/theme"
	"tableflip.dev/bujo/pkg/tui/ui"
	"tableflip.dev/bujo/pkg/tui/uiutil"
)

// Ensure component contract compliance.
var _ ui.Component = (*Model)(nil)

// Focus identifies which column is focused in the migration view.
type Focus int

const (
	FocusTasks Focus = iota
	FocusTargets
)

// Item captures a migration candidate entry.
type Item struct {
	Entry       *entry.Entry
	Parent      *entry.Entry
	LastTouched time.Time
}

// Model renders the migration dashboard view.
type Model struct {
	Active bool

	Label string
	Since time.Time
	Until time.Time

	Items  []Item
	Index  int
	Scroll int

	Targets       []string
	TargetMetas   map[string]collection.Meta
	MonthChildren map[string][]string
	TargetIndex   int
	TargetScroll  int

	Focus         Focus
	MigratedCount int

	width  int
	height int

	theme        theme.Theme
	relativeTime func(time.Time, time.Time) string
}

// New constructs a migration view model.
func New(th theme.Theme, relative func(time.Time, time.Time) string) *Model {
	if relative == nil {
		relative = func(time.Time, time.Time) string { return "" }
	}
	return &Model{
		theme:        th,
		relativeTime: relative,
	}
}

// Init implements ui.Component.
func (m *Model) Init() tea.Cmd { return nil }

// Update implements ui.Component. The migration view does not emit commands directly.
func (m *Model) Update(msg tea.Msg) (ui.Component, tea.Cmd) { return m, nil }

// SetSize stores the width/height available for rendering.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// View renders the migration dashboard.
func (m *Model) View() string {
	if !m.Active {
		return ""
	}
	label := "Migration"
	if strings.TrimSpace(m.Label) != "" {
		label = fmt.Sprintf("Migration · last %s", m.Label)
	}
	remaining := len(m.Items)
	label = fmt.Sprintf("%s (%d remaining · %d migrated)", label, remaining, m.MigratedCount)

	totalWidth := m.width
	if totalWidth <= 0 {
		totalWidth = 80
	}
	separatorWidth := 2
	rightWidth := m.collectionPaneWidth()
	if rightWidth < 20 {
		rightWidth = 20
	}
	if rightWidth > totalWidth/2 {
		rightWidth = totalWidth / 3
	}
	leftWidth := totalWidth - separatorWidth - rightWidth
	if leftWidth < 40 {
		diff := 40 - leftWidth
		leftWidth = 40
		if rightWidth-diff >= 18 {
			rightWidth -= diff
		} else {
			rightWidth = 18
		}
	}
	if rightWidth < 18 {
		rightWidth = 18
		leftWidth = totalWidth - separatorWidth - rightWidth
	}
	if leftWidth < 20 {
		leftWidth = 20
	}
	height := m.height
	if height <= 0 {
		height = 20
	}

	left := m.renderLeft(leftWidth, height)
	right := m.renderRight(rightWidth, height)
	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(separatorWidth).
		Height(height).
		Render(strings.Repeat("│", height))
	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(leftWidth).Render(left),
		separator,
		lipgloss.NewStyle().Width(rightWidth).Render(right),
	)
	return label + "\n\n" + body
}

// CurrentItem returns the focused migration item (if any).
func (m *Model) CurrentItem() *Item {
	if !m.Active || len(m.Items) == 0 {
		return nil
	}
	if m.Index < 0 {
		m.Index = 0
	}
	if m.Index >= len(m.Items) {
		m.Index = len(m.Items) - 1
	}
	return &m.Items[m.Index]
}

// CurrentTarget returns the focused collection target path.
func (m *Model) CurrentTarget() string {
	if len(m.Targets) == 0 {
		return ""
	}
	if m.TargetIndex < 0 {
		m.TargetIndex = 0
	}
	if m.TargetIndex >= len(m.Targets) {
		m.TargetIndex = len(m.Targets) - 1
	}
	return m.Targets[m.TargetIndex]
}

// RemoveItem removes the entry with the specified ID, maintaining scroll bounds.
func (m *Model) RemoveItem(id string) bool {
	if strings.TrimSpace(id) == "" || len(m.Items) == 0 {
		return false
	}
	idx := -1
	for i, it := range m.Items {
		if it.Entry != nil && it.Entry.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	m.Items = append(m.Items[:idx], m.Items[idx+1:]...)
	if len(m.Items) == 0 {
		m.Index = 0
		m.Scroll = 0
		return true
	}
	if m.Index >= len(m.Items) {
		m.Index = len(m.Items) - 1
	}
	if m.Index < 0 {
		m.Index = 0
	}
	m.EnsureTaskVisible()
	return true
}

// EnsureTaskVisible adjusts the task list scroll to keep the focused item visible.
func (m *Model) EnsureTaskVisible() {
	if len(m.Items) == 0 {
		m.Scroll = 0
		return
	}
	slots := m.itemsVisibleSlots()
	if m.Index < m.Scroll {
		m.Scroll = m.Index
	}
	if m.Index >= m.Scroll+slots {
		m.Scroll = m.Index - slots + 1
	}
	maxScroll := len(m.Items) - slots
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.Scroll > maxScroll {
		m.Scroll = maxScroll
	}
	if m.Scroll < 0 {
		m.Scroll = 0
	}
}

// EnsureTargetVisible keeps the focused target visible.
func (m *Model) EnsureTargetVisible() {
	if len(m.Targets) == 0 {
		m.TargetScroll = 0
		return
	}
	slots := m.targetsVisibleSlots()
	if m.TargetIndex < m.TargetScroll {
		m.TargetScroll = m.TargetIndex
	}
	if m.TargetIndex >= m.TargetScroll+slots {
		m.TargetScroll = m.TargetIndex - slots + 1
	}
	max := len(m.Targets) - slots
	if max < 0 {
		max = 0
	}
	if m.TargetScroll > max {
		m.TargetScroll = max
	}
	if m.TargetScroll < 0 {
		m.TargetScroll = 0
	}
}

func (m *Model) renderLeft(width, height int) string {
	if len(m.Items) == 0 {
		placeholder := lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Render("<no open tasks in this window>")
		return lipgloss.NewStyle().Width(width).Height(height).Render(placeholder)
	}
	now := time.Now()
	lines := make([]string, 0, height)
	start := m.Scroll
	slots := m.itemsVisibleSlots()
	end := start + slots
	if end > len(m.Items) {
		end = len(m.Items)
	}
	prevCollection := ""
	headerStyle := m.theme.Panel.Title
	for i := start; i < end; i++ {
		item := m.Items[i]
		name := uiutil.FormattedCollectionName(item.Entry.Collection)
		if strings.TrimSpace(name) == "" {
			name = item.Entry.Collection
		}
		if i == start || item.Entry.Collection != prevCollection {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, headerStyle.Render(name))
			prevCollection = item.Entry.Collection
		}
		highlight := i == m.Index && m.Focus == FocusTasks
		caret := "  "
		style := m.theme.Panel.Body
		selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
		if highlight {
			caret = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Render("→ ")
			style = selectedStyle
		}
		if item.Parent != nil {
			parentLine := "  ▸ " + uiutil.EntryLabel(item.Parent)
			if highlight {
				parentLine = style.Render(parentLine)
			}
			lines = append(lines, parentLine)
		}
		signifier := strings.TrimSpace(item.Entry.Signifier.Glyph().Symbol)
		if signifier == "" {
			signifier = " "
		}
		bullet := strings.TrimSpace(item.Entry.Bullet.Glyph().Symbol)
		if bullet == "" {
			bullet = item.Entry.Bullet.String()
		}
		message := strings.TrimSpace(item.Entry.Message)
		if message == "" {
			message = "<empty>"
		}
		touched := m.relativeTime(item.LastTouched, now)
		entryLine := fmt.Sprintf("%s%s %s %s  · %s", caret, signifier, bullet, message, touched)
		if highlight {
			entryLine = style.Render(entryLine)
		}
		lines = append(lines, entryLine)
	}
	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().Width(width).Height(height).Render(content)
}

func (m *Model) renderRight(width, height int) string {
	lines := make([]string, 0, height+8)
	headerStyle := m.theme.Panel.Title
	bodyStyle := m.theme.Panel.Body
	lines = append(lines, headerStyle.Render("Collections"))

	if len(m.Targets) == 0 {
		lines = append(lines, bodyStyle.Render("  <no collections available>"))
	} else {
		start := m.TargetScroll
		slots := m.targetsVisibleSlots()
		end := start + slots
		if end > len(m.Targets) {
			end = len(m.Targets)
		}
		selectedTarget := ""
		if m.Focus == FocusTargets && m.TargetIndex >= 0 && m.TargetIndex < len(m.Targets) {
			selectedTarget = m.Targets[m.TargetIndex]
		}
		selectedParent := uiutil.ParentCollectionName(selectedTarget)
		selectedDay := uiutil.ParseDayNumber(selectedParent, selectedTarget)

		for i := start; i < end; i++ {
			target := m.Targets[i]
			meta := m.TargetMetas[target]
			display := uiutil.FormattedCollectionName(target)
			if strings.TrimSpace(display) == "" {
				display = target
			}
			indent := strings.Count(target, "/")
			highlight := m.Focus == FocusTargets && i == m.TargetIndex
			prefix := strings.Repeat("  ", indent)
			style := bodyStyle
			selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)

			if meta.Type == collection.TypeDaily && indent == 0 {
				monthSelected := highlight || (selectedParent == target && selectedDay > 0)
				if monthSelected {
					prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Render("→ ") + prefix
					style = selectedStyle
				} else {
					prefix = "  " + prefix
				}
				text := display
				if monthSelected {
					text = style.Render(text)
				}
				lines = append(lines, prefix+text)
				selDay := 0
				if selectedParent == target {
					selDay = selectedDay
				}
				calLines := m.renderCalendar(target, m.MonthChildren[target], selDay)
				for _, cal := range calLines {
					lines = append(lines, "    "+cal)
				}
				continue
			}

			if highlight {
				prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Render("→ ") + prefix
				style = selectedStyle
			} else {
				prefix = "  " + prefix
			}
			text := display
			if highlight {
				text = style.Render(text)
			}
			lines = append(lines, prefix+text)
		}
		if end < len(m.Targets) {
			lines = append(lines, bodyStyle.Render(fmt.Sprintf("  … (%d more)", len(m.Targets)-end)))
		}
	}

	renderLines := lines
	if len(renderLines) > height {
		renderLines = renderLines[:height]
	}
	render := strings.Join(renderLines, "\n")
	return lipgloss.NewStyle().Width(width).Height(height).Render(render)
}

func (m *Model) renderCalendar(monthPath string, children []string, selectedDay int) []string {
	monthName := uiutil.LastSegment(monthPath)
	monthName = strings.ReplaceAll(monthName, ",", "")
	if monthName == "" {
		monthName = strings.TrimSpace(monthPath)
	}
	monthTime, ok := index.ParseMonth(monthName)
	if !ok {
		return nil
	}
	childItems := make([]index.CollectionItem, 0, len(children))
	for _, child := range children {
		childItems = append(childItems, index.CollectionItem{Name: uiutil.LastSegment(child), Resolved: child})
	}
	header, weeks := index.RenderCalendarRows(monthName, monthTime, childItems, selectedDay, time.Now(), index.DefaultCalendarOptions())
	if header == nil {
		return nil
	}
	lines := []string{header.Text}
	for _, week := range weeks {
		lines = append(lines, week.Text)
	}
	return lines
}

func (m *Model) collectionPaneWidth() int {
	left := m.width / 3
	if left < 24 {
		left = 24
	}
	if left > 40 {
		left = 40
	}
	if left <= 0 {
		left = 24
	}
	return left
}

func (m *Model) itemsVisibleSlots() int {
	height := m.height
	if height <= 0 {
		height = 20
	}
	linesPerItem := 3
	slots := height / linesPerItem
	if slots < 1 {
		slots = 1
	}
	return slots
}

func (m *Model) targetsVisibleSlots() int {
	height := m.height
	if height <= 0 {
		height = 20
	}
	slots := height / 3
	if slots < 5 {
		slots = 5
	}
	return slots
}
