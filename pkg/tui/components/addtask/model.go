package addtask

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/glyph"
	cachepkg "tableflip.dev/bujo/pkg/tui/cache"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
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
	InitialCollectionID   string
	InitialParentBulletID string
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
	cache   *cachepkg.Cache
	id      events.ComponentID
	focused bool

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
func NewModel(cache *cachepkg.Cache, opts Options) *Model {
	tInput := textinput.New()
	tInput.Placeholder = "Describe the task…"
	tInput.Focus()
	tInput.Prompt = ""

	m := &Model{
		cache:     cache,
		id:        events.ComponentID("addtask"),
		focused:   true,
		focus:     fieldTaskInput,
		taskInput: tInput,
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
	if opts.InitialCollectionID != "" {
		m.selectCollectionByID(opts.InitialCollectionID)
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

func (m *Model) blurCmd() tea.Cmd {
	if !m.focused {
		return nil
	}
	m.focused = false
	m.updateInputFocus()
	return events.BlurCmd(m.id)
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

// View renders the overlay UI.
func (m *Model) View() (string, *tea.Cursor) {
	lines := []string{m.sectionTitle("Add Task")}
	lines = append(lines, m.renderCollectionRow())
	lines = append(lines, m.renderParentRow(), "")

	controlRow, controlPrefix := m.renderControlRow()
	controlRowIndex := len(lines)
	lines = append(lines, controlRow, "", m.renderStatusLine())

	bodyContent := lipgloss.JoinVertical(lipgloss.Left, lines...)
	maxContent := m.width - 12
	if maxContent < 20 {
		maxContent = m.width - 8
	}
	if maxContent < 16 {
		maxContent = 16
	}
	contentWidth := clampInt(m.fieldWidth, 16, maxContent)
	body := lipgloss.NewStyle().Width(contentWidth).Render(bodyContent)

	var cursor *tea.Cursor
	if c := m.taskInput.Cursor(); c != nil {
		clone := *c
		clone.Position.X += controlPrefix
		clone.Position.Y += controlRowIndex
		cursor = &clone
	}

	frameStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("212")).
		Padding(1, 2)
	if !m.focused {
		frameStyle = frameStyle.BorderForeground(lipgloss.Color("240"))
	}
	box := frameStyle.Render(body)

	if cursor != nil {
		cursor.Position.X += 2 // left padding
		cursor.Position.Y += 1 // top padding
		cursor.Position.X += 1 // left border
		cursor.Position.Y += 1 // top border
	}

	return box, cursor
}

// SetSize configures the overlay dimensions.
func (m *Model) SetSize(width, height int) {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 20
	}
	m.width = width
	m.height = height
	usable := width - 8
	if usable < 14 {
		usable = width - 6
	}
	if usable < 12 {
		usable = 12
	}
	m.fieldWidth = usable
	inputWidth := usable - 16
	if inputWidth < 12 {
		inputWidth = max(10, usable-4)
	}
	m.taskInput.SetWidth(inputWidth)
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	if m.confirmReset {
		switch msg.String() {
		case "y", "enter":
			m.resetForm()
			m.confirmReset = false
			return m.blurCmd()
		case "n", "esc":
			m.confirmReset = false
		}
		return nil
	}

	if !m.focused {
		return nil
	}

	var cmds []tea.Cmd

	switch msg.String() {
	case "tab", "shift+tab":
		if msg.String() == "tab" {
			m.advanceFocus(1)
		} else {
			m.advanceFocus(-1)
		}
	case "up", "k":
		m.adjustSelection(-1)
	case "down", "j":
		m.adjustSelection(1)
	case "left", "h":
		m.adjustSelection(-1)
	case "right", "l":
		m.adjustSelection(1)
	case "enter":
		if cmd, err := m.submit(); err != nil {
			m.errorMsg = err.Error()
		} else if cmd != nil {
			cmds = appendCmd(cmds, cmd)
		}
	case "esc":
		m.confirmReset = true
	default:
		// delegate to focused input
		switch m.focus {
		case fieldTaskInput:
			var cmd tea.Cmd
			m.taskInput, cmd = m.taskInput.Update(msg)
			cmds = appendCmd(cmds, cmd)
		}
	}
	cmds = appendCmd(cmds, m.updateInputFocus())
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func appendCmd(cmds []tea.Cmd, cmd tea.Cmd) []tea.Cmd {
	if cmd == nil {
		return cmds
	}
	return append(cmds, cmd)
}

func (m *Model) updateInputFocus() tea.Cmd {
	if !m.focused {
		m.taskInput.Blur()
		return nil
	}
	if m.focus == fieldTaskInput {
		return m.taskInput.Focus()
	}
	m.taskInput.Blur()
	return nil
}

func (m *Model) advanceFocus(delta int) {
	seq := m.focusSequence()
	if len(seq) == 0 {
		return
	}
	current := 0
	for i, f := range seq {
		if f == m.focus {
			current = i
			break
		}
	}
	current = (current + len(seq) + delta) % len(seq)
	m.focus = seq[current]
	m.updateInputFocus()
}

func (m *Model) focusSequence() []focusField {
	return []focusField{
		fieldParentBullet,
		fieldBulletType,
		fieldSignifier,
		fieldTaskInput,
	}
}

func (m *Model) adjustSelection(delta int) {
	switch m.focus {
	case fieldParentBullet:
		if len(m.parentOptions) == 0 {
			return
		}
		m.parentIndex = clampIndex(m.parentIndex+delta, len(m.parentOptions))
	case fieldBulletType:
		if len(m.bulletOptions) == 0 {
			return
		}
		m.bulletIndex = clampIndex(m.bulletIndex+delta, len(m.bulletOptions))
		m.updatePrompt()
	case fieldSignifier:
		if len(m.signifierOptions) == 0 {
			return
		}
		m.signifierIndex = clampIndex(m.signifierIndex+delta, len(m.signifierOptions))
		m.updatePrompt()
	}
}

func (m *Model) submit() (tea.Cmd, error) {
	label := strings.TrimSpace(m.taskInput.Value())
	if label == "" {
		return nil, fmt.Errorf("task description is required")
	}
	targetID := m.resolveTargetCollection()
	if targetID == "" {
		return nil, fmt.Errorf("select a collection first")
	}
	parentID := ""
	if m.parentIndex > 0 && m.parentIndex < len(m.parentOptions) {
		parentID = m.parentOptions[m.parentIndex].ID
	}
	bulletID := fmt.Sprintf("tmp-%d", time.Now().UnixNano())
	bullet := collectiondetail.Bullet{
		ID:        bulletID,
		Label:     label,
		Bullet:    m.bulletOptions[m.bulletIndex],
		Signifier: m.signifierOptions[m.signifierIndex],
		Created:   time.Now(),
	}

	metaMap := map[string]string{}
	if parentID != "" {
		metaMap[cacheParentMetaKey] = parentID
	}
	if len(metaMap) == 0 {
		metaMap = nil
	}

	m.cache.CreateBulletWithMeta(targetID, bullet, metaMap)
	m.resetForm()
	m.lastSubmitted = time.Now()
	return m.blurCmd(), nil
}

func (m *Model) resetForm() {
	m.taskInput.SetValue("")
	m.parentIndex = 0
	m.errorMsg = ""
	m.confirmReset = false
	m.refreshParentOptions()
}

func (m *Model) updatePrompt() {
	bullet := m.bulletOptions[m.bulletIndex]
	signifier := m.signifierOptions[m.signifierIndex]
	prompt := describePrompt(bullet, signifier, m.currentParentLabel())
	m.taskInput.Placeholder = prompt
}

func (m *Model) currentParentLabel() string {
	if m.parentIndex > 0 && m.parentIndex < len(m.parentOptions) {
		return m.parentOptions[m.parentIndex].Label
	}
	return ""
}

func (m *Model) resolveTargetCollection() string {
	if len(m.collectionOptions) == 0 {
		return ""
	}
	if m.collectionIndex >= len(m.collectionOptions) {
		m.collectionIndex = len(m.collectionOptions) - 1
	}
	if m.collectionIndex < 0 {
		m.collectionIndex = 0
	}
	meta := m.collectionOptions[m.collectionIndex].Meta
	return meta.Name
}

func (m *Model) refreshCollections() {
	metas := m.cache.CollectionsMeta()
	currentID := ""
	if len(m.collectionOptions) > 0 && m.collectionIndex >= 0 && m.collectionIndex < len(m.collectionOptions) {
		currentID = m.collectionOptions[m.collectionIndex].ID
	}
	if len(metas) == 0 {
		m.collectionOptions = nil
		m.collectionIndex = 0
		return
	}
	opts := make([]collectionOption, 0, len(metas))
	for _, meta := range metas {
		opts = append(opts, collectionOption{
			ID:    meta.Name,
			Label: collectionLabel(meta.Name),
			Meta:  meta,
		})
	}
	m.collectionOptions = opts
	if currentID != "" {
		m.selectCollectionByID(currentID)
	} else {
		m.collectionIndex = clampIndex(m.collectionIndex, len(m.collectionOptions))
	}
	m.updatePrompt()
}

func (m *Model) refreshParentOptions() {
	target := m.resolveTargetCollection()
	if target == "" {
		m.parentOptions = []parentOption{{ID: "", Label: "(none)"}}
		m.parentIndex = 0
		m.updatePrompt()
		return
	}
	section, ok := m.cache.SectionSnapshot(target)
	if !ok {
		m.parentOptions = []parentOption{{ID: "", Label: "(none)"}}
		m.parentIndex = 0
		m.updatePrompt()
		return
	}
	opts := []parentOption{{ID: "", Label: "(none)"}}
	for _, bullet := range section.Bullets {
		label := bullet.Label
		if strings.TrimSpace(label) == "" {
			label = bullet.ID
		}
		opts = append(opts, parentOption{
			ID:    bullet.ID,
			Label: label,
		})
	}
	m.parentOptions = opts
	m.parentIndex = clampIndex(m.parentIndex, len(m.parentOptions))
	m.updatePrompt()
}

func (m *Model) selectCollectionByID(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	for idx, opt := range m.collectionOptions {
		if strings.EqualFold(opt.ID, id) {
			m.collectionIndex = idx
			return
		}
	}
}

func (m *Model) selectParentByID(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		m.parentIndex = 0
		return
	}
	for idx, opt := range m.parentOptions {
		if strings.EqualFold(opt.ID, id) {
			m.parentIndex = idx
			return
		}
	}
}

func (m *Model) renderStatusLine() string {
	switch {
	case m.confirmReset:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Render("Press 'y' to discard draft, 'n' to continue editing.")
	case m.errorMsg != "":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Render(m.errorMsg)
	default:
		return "Enter to submit • Esc to clear • Tab between fields"
	}
}

func (m *Model) sectionTitle(title string) string {
	return lipgloss.NewStyle().Bold(true).Render(title)
}

func (m *Model) renderCollectionRow() string {
	if len(m.collectionOptions) == 0 {
		return m.renderRow("Collection:", "(none)", false)
	}
	if m.collectionIndex >= len(m.collectionOptions) {
		m.collectionIndex = len(m.collectionOptions) - 1
	}
	if m.collectionIndex < 0 {
		m.collectionIndex = 0
	}
	value := m.collectionOptions[m.collectionIndex].Label
	return m.renderRow("Collection:", value, false)
}

func (m *Model) renderParentRow() string {
	value := "(none)"
	if len(m.parentOptions) > 0 && m.parentIndex < len(m.parentOptions) {
		value = m.parentOptions[m.parentIndex].Label
	}
	return m.renderRow("Parent bullet:", value, m.focus == fieldParentBullet)
}

func (m *Model) renderControlRow() (string, int) {
	signifier := m.signifierOptions[m.signifierIndex]
	signifierLabel := symbolForSignifier(signifier)
	signifierBox := m.renderBox(signifierLabel, m.focus == fieldSignifier)

	bullet := m.bulletOptions[m.bulletIndex]
	bulletLabel := symbolForBullet(bullet)
	bulletBox := m.renderBox(bulletLabel, m.focus == fieldBulletType)

	cursorGlyph := "➤"
	if m.focus == fieldTaskInput && m.focused {
		cursorGlyph = lipgloss.NewStyle().Foreground(focusColor).Render(cursorGlyph)
	}

	prefix := fmt.Sprintf("  %s %s %s ", signifierBox, bulletBox, cursorGlyph)
	line := prefix + m.taskInput.View()
	return line, lipgloss.Width(prefix)
}

func (m *Model) renderRow(label, value string, focused bool) string {
	indicator := "  "
	labelStyle := lipgloss.NewStyle().Bold(false)
	valueStyle := lipgloss.NewStyle()
	if focused {
		style := lipgloss.NewStyle().Foreground(focusColor)
		indicator = style.Render("➤ ")
		labelStyle = labelStyle.Foreground(focusColor)
		valueStyle = valueStyle.Foreground(focusColor)
	}
	return indicator + labelStyle.Render(fmt.Sprintf("%-13s", label)) + " " + valueStyle.Render(value)
}

func (m *Model) renderBox(content string, focused bool) string {
	if strings.TrimSpace(content) == "" {
		content = " "
	}
	box := fmt.Sprintf("[%s]", content)
	if focused {
		return lipgloss.NewStyle().Foreground(focusColor).Render(box)
	}
	return box
}

func symbolForBullet(b glyph.Bullet) string {
	if info, ok := glyph.DefaultBullets()[b]; ok {
		if s := strings.TrimSpace(info.Symbol); s != "" {
			return s
		}
	}
	return string(b)
}

func symbolForSignifier(s glyph.Signifier) string {
	if info, ok := glyph.DefaultSignifiers()[s]; ok {
		if sym := strings.TrimSpace(info.Symbol); sym != "" {
			return sym
		}
	}
	return " "
}

func clampIndex(value, length int) int {
	if length <= 0 {
		return 0
	}
	if value < 0 {
		return 0
	}
	if value >= length {
		return length - 1
	}
	return value
}

func collectionLabel(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "(unnamed)"
	}
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}

const cacheParentMetaKey = "parent_id"

func clampInt(value, minVal, maxVal int) int {
	if maxVal > 0 && value > maxVal {
		value = maxVal
	}
	if value < minVal {
		value = minVal
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func describePrompt(b glyph.Bullet, s glyph.Signifier, parent string) string {
	base := "Describe the task..."
	parent = strings.ToLower(parent)
	switch b {
	case glyph.Event:
		base = "Describe the event..."
	case glyph.Note:
		base = "Describe the note..."
	}
	if strings.Contains(parent, "event") {
		base = strings.Replace(base, "task", "event", 1)
	}
	if strings.Contains(parent, "note") {
		base = strings.Replace(base, "task", "note", 1)
	}
	switch s {
	case glyph.Priority:
		return strings.Replace(base, "Describe", "Describe the important", 1)
	case glyph.Inspiration:
		return strings.Replace(base, "Describe", "Describe the inspiration", 1)
	case glyph.Investigation:
		return strings.Replace(base, "Describe", "Describe the investigation", 1)
	default:
		return base
	}
}
