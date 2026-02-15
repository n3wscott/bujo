package collectiondetail2

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/glyph"
	base "tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/events"
	"tableflip.dev/bujo/pkg/tui/uiutil"
)

// Bullet describes a single entry row inside a collection detail section.
type Bullet = base.Bullet

// Section groups a set of bullets under a collection title.
type Section = base.Section

// Mode controls how the detail pane renders its sections.
type Mode int

const (
	// ModeContinuous renders all sections in a single scrollable detail pane.
	ModeContinuous Mode = iota
	// ModeFocused renders only the currently active section.
	ModeFocused
)

// Model renders a scrollable list of section headers with their bullets.
type Model struct {
	sections []Section
	lookup   map[string]int

	width    int
	height   int
	debugLog io.Writer
	keys     keyMap

	mode             Mode
	cursor           int // index into sectionBulletLines[activeSection], -1 when nothing selectable
	scroll           int
	activeSection    int
	pendingSectionID string
	pendingBulletID  string

	focused bool

	lines              []lineInfo
	sectionBulletLines [][]int
	lineHeights        []int
	lineOffsets        []int
	totalHeight        int
	sectionCursors     map[string]int
	id                 events.ComponentID
	lastHighlight      string
	sourceNav          events.ComponentID
}

const (
	lineHeader = -1
	lineSpacer = -2
	lineEmpty  = -3
	lineItem   = -4
)

type lineInfo struct {
	section int
	kind    int // >=0 bullet index, otherwise line constants above
	indent  int
	bullet  Bullet
}

// NewModel constructs the detail component with the provided sections.
func NewModel(sections []Section) *Model {
	m := &Model{
		cursor:        -1,
		activeSection: -1,
		id:            events.ComponentID("collectiondetail2"),
		keys:          defaultKeyMap(),
		mode:          ModeContinuous,
	}
	m.SetSections(sections)
	return m
}

// SetDebugWriter configures an optional writer for diagnostic output.
func (m *Model) SetDebugWriter(w io.Writer) {
	m.debugLog = w
}

// SetMode switches between continuous and focused rendering.
func (m *Model) SetMode(mode Mode) bool {
	if mode != ModeContinuous && mode != ModeFocused {
		return false
	}
	if m.mode == mode {
		return false
	}
	m.mode = mode
	m.refreshFromSections(false)
	return true
}

// Mode returns the current render mode.
func (m *Model) Mode() Mode {
	return m.mode
}

// SetSections replaces the rendered sections.
func (m *Model) SetSections(sections []Section) {
	m.sections = append([]Section(nil), sections...)
	for i := range m.sections {
		if len(m.sections[i].Bullets) > 0 {
			m.sections[i].Placeholder = false
		}
	}
	m.rebuildLookup()
	m.refreshFromSections(true)
}

func (m *Model) rebuildLookup() {
	if m.lookup == nil {
		m.lookup = make(map[string]int)
	} else {
		for k := range m.lookup {
			delete(m.lookup, k)
		}
	}
	for idx, sec := range m.sections {
		if sec.ID != "" {
			m.lookup["id:"+strings.ToLower(sec.ID)] = idx
		}
		if sec.Title != "" {
			m.lookup["title:"+strings.ToLower(sec.Title)] = idx
		}
	}
}

func (m *Model) refreshFromSections(resetHighlight bool) {
	m.rebuildLines()
	m.clampActiveSection()
	m.cursor = m.cursorForSection(m.activeSection)
	m.applyPendingSelection()
	m.ensureScroll()
	if resetHighlight {
		m.lastHighlight = ""
	}
}

// SetSourceNav configures which nav component drives highlight events for this
// detail pane. When empty, all highlight events are accepted.
func (m *Model) SetSourceNav(id events.ComponentID) {
	m.sourceNav = id
}

// SetSize configures the viewport dimensions.
func (m *Model) SetSize(width, height int) {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 20
	}
	m.width = width
	m.height = height
	m.recomputeLineMetrics()
	m.ensureScroll()
}

// Focus marks the component as active (highlights the cursor line).
func (m *Model) Focus() tea.Cmd {
	if m.focused {
		return nil
	}
	m.focused = true
	return events.FocusCmd(m.id)
}

// Blur marks the component as inactive.
func (m *Model) Blur() tea.Cmd {
	if !m.focused {
		return nil
	}
	m.focused = false
	return events.BlurCmd(m.id)
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// Update handles key presses for navigation.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}
		switch {
		case key.Matches(msg, m.keys.MoveUp):
			m.moveCursor(-1)
		case key.Matches(msg, m.keys.MoveDown):
			m.moveCursor(1)
		case key.Matches(msg, m.keys.PageUp):
			m.moveCursor(-m.pageSize())
		case key.Matches(msg, m.keys.PageDown):
			m.moveCursor(m.pageSize())
		case key.Matches(msg, m.keys.MoveTop):
			if lines := m.activeBulletLines(); len(lines) > 0 {
				m.cursor = 0
				m.storeCursor()
				m.ensureScroll()
			}
		case key.Matches(msg, m.keys.MoveBottom):
			if lines := m.activeBulletLines(); len(lines) > 0 {
				m.cursor = len(lines) - 1
				m.storeCursor()
				m.ensureScroll()
			}
		case key.Matches(msg, m.keys.Select):
			if cmd := m.selectCmd(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		case key.Matches(msg, m.keys.MoveCollection):
			if cmd := m.moveCmd(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		case key.Matches(msg, m.keys.Complete):
			if cmd := m.completeCmd(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		case key.Matches(msg, m.keys.Strike):
			if cmd := m.strikeCmd(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		case key.Matches(msg, m.keys.MoveFuture):
			if cmd := m.moveFutureCmd(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		case key.Matches(msg, m.keys.SignifyInvestigate):
			if cmd := m.signifierCmd(glyph.Investigation); cmd != nil {
				cmds = append(cmds, cmd)
			}
		case key.Matches(msg, m.keys.SignifyInspire):
			if cmd := m.signifierCmd(glyph.Inspiration); cmd != nil {
				cmds = append(cmds, cmd)
			}
		case key.Matches(msg, m.keys.SignifyPriority):
			if cmd := m.signifierCmd(glyph.Priority); cmd != nil {
				cmds = append(cmds, cmd)
			}
		case key.Matches(msg, m.keys.SignifyNone):
			if cmd := m.signifierCmd(glyph.None); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	case events.CollectionHighlightMsg:
		m.handleNavHighlight(msg)
	case events.CollectionChangeMsg:
		if m.applyCollectionChange(msg) {
			m.rebuildLookup()
			m.refreshFromSections(false)
		}
	case events.BulletChangeMsg:
		if m.applyBulletChange(msg) {
			m.refreshFromSections(false)
		}
	case events.CollectionSelectMsg:
		m.handleNavSelect(msg)
	case events.CollectionOrderMsg:
		if m.reorderSections(msg.Order) {
			m.rebuildLookup()
			m.refreshFromSections(false)
		}
	}

	if cmd := m.highlightCmd(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) parentForLine(lineIdx int, sectionIdx int, indent int) (Bullet, bool) {
	if lineIdx <= 0 || lineIdx > len(m.lines) {
		return Bullet{}, false
	}
	for i := lineIdx - 1; i >= 0; i-- {
		info := m.lines[i]
		if info.kind != lineItem {
			continue
		}
		if info.section != sectionIdx {
			continue
		}
		if info.indent < indent {
			if strings.TrimSpace(info.bullet.ID) == "" {
				return Bullet{}, false
			}
			return info.bullet, true
		}
	}
	return Bullet{}, false
}

func (m *Model) sectionActive(section int) bool {
	return section >= 0 && section < len(m.sections) && section == m.activeSection
}

func (m *Model) sectionIndexForCollection(ref events.CollectionRef) int {
	if idx := m.lookupIndex("id", ref.ID); idx >= 0 {
		return idx
	}
	if idx := m.lookupIndex("title", ref.Name); idx >= 0 {
		return idx
	}
	return -1
}

func (m *Model) sectionIndexForView(ref events.CollectionViewRef) int {
	if idx := m.lookupIndex("id", ref.ID); idx >= 0 {
		return idx
	}
	if idx := m.lookupIndex("title", ref.Title); idx >= 0 {
		return idx
	}
	return -1
}

func (m *Model) lookupIndex(kind, value string) int {
	if value == "" || m.lookup == nil {
		return -1
	}
	if idx, ok := m.lookup[kind+":"+strings.ToLower(value)]; ok {
		return idx
	}
	return -1
}

func (m *Model) clampActiveSection() {
	if len(m.sections) == 0 {
		m.activeSection = -1
		return
	}
	if m.activeSection < 0 || m.activeSection >= len(m.sections) {
		m.activeSection = 0
	}
}

func (m *Model) activeBulletLines() []int {
	if m.activeSection < 0 || m.activeSection >= len(m.sectionBulletLines) {
		return nil
	}
	return m.sectionBulletLines[m.activeSection]
}

func (m *Model) cursorForSection(sectionIdx int) int {
	if sectionIdx < 0 || sectionIdx >= len(m.sectionBulletLines) {
		return -1
	}
	lines := m.sectionBulletLines[sectionIdx]
	if len(lines) == 0 {
		return -1
	}
	if m.sectionCursors == nil {
		return 0
	}
	key := sectionOrderKey(m.sections[sectionIdx].ID)
	if key == "" {
		return 0
	}
	if cursor, ok := m.sectionCursors[key]; ok {
		if cursor < 0 {
			return -1
		}
		if cursor >= len(lines) {
			return len(lines) - 1
		}
		return cursor
	}
	return 0
}

func (m *Model) storeCursor() {
	if m.sectionCursors == nil {
		m.sectionCursors = make(map[string]int)
	}
	if m.activeSection < 0 || m.activeSection >= len(m.sections) {
		return
	}
	key := sectionOrderKey(m.sections[m.activeSection].ID)
	if key == "" {
		return
	}
	m.sectionCursors[key] = m.cursor
}

func (m *Model) setActiveSection(sectionIdx int) {
	if sectionIdx < 0 || sectionIdx >= len(m.sections) {
		return
	}
	if m.activeSection == sectionIdx {
		return
	}
	m.storeCursor()
	m.activeSection = sectionIdx
	if m.mode == ModeFocused {
		m.rebuildLines()
	}
	m.cursor = m.cursorForSection(sectionIdx)
}

func (m *Model) focusSectionForCollection(ref events.CollectionRef) bool {
	sectionIdx := m.sectionIndexForCollection(ref)
	if sectionIdx < 0 {
		return false
	}
	m.setActiveSection(sectionIdx)
	if lines := m.activeBulletLines(); len(lines) > 0 && m.cursor < 0 {
		m.cursor = 0
		m.storeCursor()
	}
	targetLine := -1
	for idx, info := range m.lines {
		if info.section == sectionIdx && info.kind == lineHeader {
			targetLine = idx
			break
		}
	}
	if targetLine >= 0 {
		m.scrollToLine(targetLine)
	} else {
		m.ensureScroll()
	}
	return true
}

func (m *Model) scrollToLine(line int) {
	if line < 0 || line >= len(m.lines) {
		return
	}
	m.ensureLineVisible(line)
}

func (m *Model) currentBulletInfo() (lineInfo, Section, bool) {
	lineIdx := m.currentLineIndex()
	if lineIdx < 0 || lineIdx >= len(m.lines) {
		return lineInfo{}, Section{}, false
	}
	info := m.lines[lineIdx]
	if info.kind != lineItem || info.section < 0 || info.section >= len(m.sections) {
		return lineInfo{}, Section{}, false
	}
	return info, m.sections[info.section], true
}

// CurrentSelection returns the active section and, when available, the
// highlighted bullet. The boolean result reports whether a section is active.
func (m *Model) CurrentSelection() (Section, Bullet, bool) {
	if len(m.sections) == 0 {
		return Section{}, Bullet{}, false
	}
	if info, section, ok := m.currentBulletInfo(); ok {
		return section, info.bullet, true
	}
	secIdx := m.activeSection
	if secIdx < 0 || secIdx >= len(m.sections) {
		secIdx = 0
	}
	return m.sections[secIdx], Bullet{}, true
}

// CurrentSelectionWithParent returns the active section, highlighted bullet, and
// the parent bullet metadata when available.
func (m *Model) CurrentSelectionWithParent() (Section, Bullet, string, string, bool) {
	if len(m.sections) == 0 {
		return Section{}, Bullet{}, "", "", false
	}
	info, section, ok := m.currentBulletInfo()
	if !ok {
		secIdx := m.activeSection
		if secIdx < 0 || secIdx >= len(m.sections) {
			secIdx = 0
		}
		return m.sections[secIdx], Bullet{}, "", "", true
	}
	parentID := ""
	parentLabel := ""
	if lineIdx := m.currentLineIndex(); lineIdx >= 0 {
		if parent, found := m.parentForLine(lineIdx, info.section, info.indent); found {
			parentID = strings.TrimSpace(parent.ID)
			parentLabel = strings.TrimSpace(parent.Label)
			if parentLabel == "" {
				parentLabel = parentID
			}
		}
	}
	return section, info.bullet, parentID, parentLabel, true
}

// FocusCollection moves the cursor so the requested collection is visible and active.
func (m *Model) FocusCollection(collectionID string) {
	collectionID = strings.TrimSpace(collectionID)
	if collectionID == "" {
		return
	}
	if m.focusBulletByID(collectionID, "") {
		return
	}
	if m.focusSectionByID(collectionID) {
		return
	}
	m.pendingSectionID = collectionID
	m.pendingBulletID = ""
}

func (m *Model) highlightCmd() tea.Cmd {
	if !m.focused {
		return nil
	}
	info, section, ok := m.currentBulletInfo()
	if !ok {
		return m.highlightEmptySectionCmd()
	}
	key := highlightKey(info)
	if key == m.lastHighlight {
		return nil
	}
	m.lastHighlight = key
	return bulletHighlightCmd(m.id, section, info.bullet)
}

func (m *Model) highlightEmptySectionCmd() tea.Cmd {
	if !m.focused {
		return nil
	}
	if m.activeSection < 0 || m.activeSection >= len(m.sections) {
		if m.lastHighlight != "" {
			m.lastHighlight = ""
		}
		return nil
	}
	key := fmt.Sprintf("section:%d", m.activeSection)
	if key == m.lastHighlight {
		return nil
	}
	m.lastHighlight = key
	return bulletHighlightCmd(m.id, m.sections[m.activeSection], Bullet{})
}

func (m *Model) selectCmd() tea.Cmd {
	info, section, ok := m.currentBulletInfo()
	if !ok {
		return nil
	}
	return bulletSelectCmd(m.id, section, info.bullet)
}

func (m *Model) moveCmd() tea.Cmd {
	info, section, ok := m.currentBulletInfo()
	if !ok {
		return nil
	}
	return events.MoveBulletRequestCmd(m.id, events.CollectionViewRef{
		ID:       section.ID,
		Title:    section.Title,
		Subtitle: section.Subtitle,
	}, events.BulletRef{
		ID:        info.bullet.ID,
		Label:     info.bullet.Label,
		Note:      info.bullet.Note,
		Bullet:    info.bullet.Bullet,
		Signifier: info.bullet.Signifier,
	})
}

func (m *Model) completeCmd() tea.Cmd {
	info, section, ok := m.currentBulletInfo()
	if !ok || strings.TrimSpace(info.bullet.ID) == "" {
		return nil
	}
	return func() tea.Msg {
		return events.BulletCompleteMsg{
			Component: m.id,
			Collection: events.CollectionViewRef{
				ID:       section.ID,
				Title:    section.Title,
				Subtitle: section.Subtitle,
			},
			Bullet: events.BulletRef{
				ID:        info.bullet.ID,
				Label:     info.bullet.Label,
				Note:      info.bullet.Note,
				Bullet:    info.bullet.Bullet,
				Signifier: info.bullet.Signifier,
			},
		}
	}
}

func (m *Model) strikeCmd() tea.Cmd {
	info, section, ok := m.currentBulletInfo()
	if !ok || strings.TrimSpace(info.bullet.ID) == "" {
		return nil
	}
	return func() tea.Msg {
		return events.BulletStrikeMsg{
			Component: m.id,
			Collection: events.CollectionViewRef{
				ID:       section.ID,
				Title:    section.Title,
				Subtitle: section.Subtitle,
			},
			Bullet: events.BulletRef{
				ID:        info.bullet.ID,
				Label:     info.bullet.Label,
				Note:      info.bullet.Note,
				Bullet:    info.bullet.Bullet,
				Signifier: info.bullet.Signifier,
			},
		}
	}
}

func (m *Model) moveFutureCmd() tea.Cmd {
	info, section, ok := m.currentBulletInfo()
	if !ok || strings.TrimSpace(info.bullet.ID) == "" {
		return nil
	}
	return func() tea.Msg {
		return events.BulletMoveFutureMsg{
			Component: m.id,
			Collection: events.CollectionViewRef{
				ID:       section.ID,
				Title:    section.Title,
				Subtitle: section.Subtitle,
			},
			Bullet: events.BulletRef{
				ID:        info.bullet.ID,
				Label:     info.bullet.Label,
				Note:      info.bullet.Note,
				Bullet:    info.bullet.Bullet,
				Signifier: info.bullet.Signifier,
			},
		}
	}
}

func (m *Model) signifierCmd(sign glyph.Signifier) tea.Cmd {
	info, section, ok := m.currentBulletInfo()
	if !ok || strings.TrimSpace(info.bullet.ID) == "" {
		return nil
	}
	return func() tea.Msg {
		return events.BulletSignifierMsg{
			Component: m.id,
			Collection: events.CollectionViewRef{
				ID:       section.ID,
				Title:    section.Title,
				Subtitle: section.Subtitle,
			},
			Bullet: events.BulletRef{
				ID:        info.bullet.ID,
				Label:     info.bullet.Label,
				Note:      info.bullet.Note,
				Bullet:    info.bullet.Bullet,
				Signifier: info.bullet.Signifier,
			},
			Signifier: sign,
		}
	}
}

func (m *Model) applyPendingSelection() {
	if m.pendingSectionID == "" {
		return
	}
	sectionID := m.pendingSectionID
	bulletID := m.pendingBulletID
	if bulletID != "" {
		if m.focusBulletByID(sectionID, bulletID) {
			m.pendingSectionID = ""
			m.pendingBulletID = ""
		}
		return
	}
	if m.focusBulletByID(sectionID, "") {
		m.pendingSectionID = ""
		return
	}
	if m.focusSectionByID(sectionID) {
		m.pendingSectionID = ""
	}
}

func (m *Model) focusBulletByID(sectionID, bulletID string) bool {
	sectionID = strings.TrimSpace(sectionID)
	if sectionID == "" {
		return false
	}
	secIdx := m.lookupIndex("id", sectionID)
	if secIdx < 0 || secIdx >= len(m.sections) {
		return false
	}
	if m.activeSection != secIdx {
		m.setActiveSection(secIdx)
	}
	lines := m.sectionBulletLines[secIdx]
	if len(lines) == 0 {
		return false
	}
	trimmedBullet := strings.TrimSpace(bulletID)
	for cursorIdx, lineIdx := range lines {
		info := m.lines[lineIdx]
		if trimmedBullet != "" {
			if info.kind == lineItem && strings.EqualFold(info.bullet.ID, trimmedBullet) {
				m.cursor = cursorIdx
				m.storeCursor()
				m.ensureScroll()
				return true
			}
			continue
		}
		if info.kind == lineItem || info.kind == lineEmpty {
			m.cursor = cursorIdx
			m.storeCursor()
			m.ensureScroll()
			return true
		}
	}
	return false
}

func (m *Model) focusSectionByID(sectionID string) bool {
	sectionID = strings.TrimSpace(sectionID)
	if sectionID == "" {
		return false
	}
	secIdx := m.lookupIndex("id", sectionID)
	if secIdx < 0 || secIdx >= len(m.sections) {
		return false
	}
	m.setActiveSection(secIdx)
	for idx, info := range m.lines {
		if info.section == secIdx && info.kind == lineHeader {
			m.scrollToLine(idx)
			break
		}
	}
	if lines := m.sectionBulletLines[secIdx]; len(lines) > 0 && m.cursor < 0 {
		m.cursor = 0
		m.storeCursor()
	}
	m.ensureScroll()
	return true
}

func highlightKey(info lineInfo) string {
	bulletID := strings.TrimSpace(info.bullet.ID)
	if bulletID != "" {
		return bulletID
	}
	return fmt.Sprintf("%d:%s:%s", info.section, info.bullet.Label, info.bullet.Note)
}

func bulletHighlightCmd(component events.ComponentID, section Section, bullet Bullet) tea.Cmd {
	sectionRef := events.CollectionViewRef{
		ID:       section.ID,
		Title:    section.Title,
		Subtitle: section.Subtitle,
	}
	bulletRef := events.BulletRef{
		ID:        bullet.ID,
		Label:     bullet.Label,
		Note:      bullet.Note,
		Bullet:    bullet.Bullet,
		Signifier: bullet.Signifier,
	}
	return func() tea.Msg {
		return events.BulletHighlightMsg{
			Component:  component,
			Collection: sectionRef,
			Bullet:     bulletRef,
		}
	}
}

func bulletSelectCmd(component events.ComponentID, section Section, bullet Bullet) tea.Cmd {
	sectionRef := events.CollectionViewRef{
		ID:       section.ID,
		Title:    section.Title,
		Subtitle: section.Subtitle,
	}
	bulletRef := events.BulletRef{
		ID:        bullet.ID,
		Label:     bullet.Label,
		Note:      bullet.Note,
		Bullet:    bullet.Bullet,
		Signifier: bullet.Signifier,
	}
	exists := bullet.ID != ""
	return func() tea.Msg {
		return events.BulletSelectMsg{
			Component:  component,
			Collection: sectionRef,
			Bullet:     bulletRef,
			Exists:     exists,
		}
	}
}

func (m *Model) ensurePlaceholderSection(ref events.CollectionRef) bool {
	idx := m.sectionIndexForCollection(ref)
	if idx >= 0 {
		if len(m.sections[idx].Bullets) == 0 && !m.sections[idx].Placeholder {
			m.sections[idx].Placeholder = true
			m.refreshFromSections(false)
			return true
		}
		return false
	}
	id := sectionIDFromRef(ref)
	title := sectionTitleFromRef(ref)
	if strings.TrimSpace(title) == "" {
		title = "(untitled)"
	}
	subtitle := strings.ToLower(strings.TrimSpace(string(ref.Type)))
	m.sections = append(m.sections, Section{
		ID:          id,
		Title:       title,
		Subtitle:    subtitle,
		Placeholder: true,
	})
	m.rebuildLookup()
	m.refreshFromSections(false)
	return true
}

func (m *Model) applyCollectionChange(msg events.CollectionChangeMsg) bool {
	switch msg.Action {
	case events.ChangeCreate:
		if idx := m.sectionIndexForCollection(msg.Current); idx >= 0 {
			return m.updateSectionFromRef(idx, msg.Current)
		}
		return m.insertSectionFromRef(msg.Current)
	case events.ChangeUpdate:
		if idx := m.sectionIndexForCollection(msg.Current); idx >= 0 {
			return m.updateSectionFromRef(idx, msg.Current)
		}
		if msg.Previous != nil {
			if idx := m.sectionIndexForCollection(*msg.Previous); idx >= 0 {
				return m.updateSectionFromRef(idx, msg.Current)
			}
		}
		return m.insertSectionFromRef(msg.Current)
	case events.ChangeDelete:
		targetIdx := m.sectionIndexForCollection(msg.Current)
		if targetIdx < 0 && msg.Previous != nil {
			targetIdx = m.sectionIndexForCollection(*msg.Previous)
		}
		if targetIdx < 0 {
			return false
		}
		m.sections = append(m.sections[:targetIdx], m.sections[targetIdx+1:]...)
		return true
	default:
		return false
	}
}

func (m *Model) reorderSections(order []string) bool {
	if len(order) == 0 || len(m.sections) <= 1 {
		return false
	}
	if m.pendingSectionID == "" {
		if info, section, ok := m.currentBulletInfo(); ok {
			m.pendingSectionID = section.ID
			if strings.TrimSpace(info.bullet.ID) != "" {
				m.pendingBulletID = info.bullet.ID
			}
		} else if m.activeSection >= 0 && m.activeSection < len(m.sections) {
			m.pendingSectionID = m.sections[m.activeSection].ID
		}
	}
	index := make(map[string]int, len(order))
	for i, id := range order {
		key := strings.ToLower(strings.TrimSpace(id))
		if key == "" {
			continue
		}
		if _, ok := index[key]; !ok {
			index[key] = i
		}
	}
	before := make([]string, len(m.sections))
	for i, sec := range m.sections {
		before[i] = sectionOrderKey(sec.ID)
	}
	sort.SliceStable(m.sections, func(i, j int) bool {
		return compareSections(m.sections[i], m.sections[j], index)
	})
	for i, sec := range m.sections {
		if before[i] != sectionOrderKey(sec.ID) {
			return true
		}
	}
	return false
}

func (m *Model) insertSectionFromRef(ref events.CollectionRef) bool {
	title := sectionTitleFromRef(ref)
	if title == "" {
		return false
	}
	if idx := m.sectionIndexForCollection(ref); idx >= 0 {
		return false
	}
	m.sections = append(m.sections, Section{
		ID:    sectionIDFromRef(ref),
		Title: title,
	})
	return true
}

func (m *Model) updateSectionFromRef(idx int, ref events.CollectionRef) bool {
	if idx < 0 || idx >= len(m.sections) {
		return false
	}
	changed := false
	id := sectionIDFromRef(ref)
	title := sectionTitleFromRef(ref)
	if id != "" && m.sections[idx].ID != id {
		m.sections[idx].ID = id
		changed = true
	}
	if title != "" && m.sections[idx].Title != title {
		m.sections[idx].Title = title
		changed = true
	}
	return changed
}

func sectionIDFromRef(ref events.CollectionRef) string {
	if ref.ID != "" {
		return strings.TrimSpace(ref.ID)
	}
	return strings.TrimSpace(ref.Label())
}

func collectionRefFromView(view events.CollectionViewRef) events.CollectionRef {
	id := strings.TrimSpace(view.ID)
	parentID := ""
	if id != "" {
		if idx := strings.LastIndex(id, "/"); idx >= 0 {
			parentID = id[:idx]
		}
	}
	name := strings.TrimSpace(view.Title)
	if name == "" {
		name = uiutil.LastSegment(id)
	}
	typ := collection.Type(strings.ToLower(strings.TrimSpace(view.Subtitle)))
	switch typ {
	case collection.TypeMonthly, collection.TypeDaily, collection.TypeTracking, collection.TypeGeneric:
	default:
		if typ == "" {
			typ = collection.TypeGeneric
		}
	}
	return events.CollectionRef{
		ID:       id,
		Name:     name,
		Type:     typ,
		ParentID: parentID,
	}
}

func sectionTitleFromRef(ref events.CollectionRef) string {
	id := sectionIDFromRef(ref)
	if formatted := uiutil.FormattedCollectionName(id); formatted != "" {
		return formatted
	}
	label := strings.TrimSpace(ref.Label())
	if label != "" {
		return label
	}
	name := strings.TrimSpace(ref.Name)
	if name != "" {
		return name
	}
	return id
}

func (m *Model) applyBulletChange(msg events.BulletChangeMsg) bool {
	sectionIdx := m.sectionIndexForView(msg.Collection)
	if sectionIdx < 0 {
		if ref := collectionRefFromView(msg.Collection); ref.ID != "" {
			if m.ensurePlaceholderSection(ref) {
				sectionIdx = m.sectionIndexForView(msg.Collection)
			}
		}
	}
	if sectionIdx < 0 {
		return false
	}
	switch msg.Action {
	case events.ChangeCreate:
		bullet := bulletFromRef(msg.Bullet)
		sec := &m.sections[sectionIdx]
		sec.Bullets = append(sec.Bullets, bullet)
		if len(sec.Bullets) > 0 {
			sec.Placeholder = false
		}
		if strings.TrimSpace(bullet.ID) != "" {
			m.pendingSectionID = sec.ID
			m.pendingBulletID = bullet.ID
		} else {
			m.pendingSectionID = sec.ID
			m.pendingBulletID = ""
		}
		return true
	case events.ChangeUpdate:
		return updateBulletInList(&m.sections[sectionIdx].Bullets, msg.Bullet)
	case events.ChangeDelete:
		return removeBulletFromList(&m.sections[sectionIdx].Bullets, msg.Bullet.ID)
	default:
		return false
	}
}

func bulletFromRef(ref events.BulletRef) Bullet {
	return Bullet{
		ID:        ref.ID,
		Label:     ref.Label,
		Note:      ref.Note,
		Bullet:    ref.Bullet,
		Signifier: ref.Signifier,
	}
}

func updateBulletInList(list *[]Bullet, updated events.BulletRef) bool {
	if updated.ID == "" {
		return false
	}
	if list == nil || len(*list) == 0 {
		return false
	}
	for idx := range *list {
		item := &(*list)[idx]
		if item.ID != "" && item.ID == updated.ID {
			mergeBullet(item, updated)
			return true
		}
		if len(item.Children) > 0 {
			if updateBulletInList(&item.Children, updated) {
				return true
			}
		}
	}
	return false
}

func mergeBullet(dst *Bullet, ref events.BulletRef) {
	if dst == nil {
		return
	}
	dst.Label = ref.Label
	dst.Note = ref.Note
	dst.Bullet = ref.Bullet
	dst.Signifier = ref.Signifier
}

func removeBulletFromList(list *[]Bullet, id string) bool {
	if list == nil || id == "" {
		return false
	}
	items := *list
	for idx := 0; idx < len(items); idx++ {
		item := items[idx]
		if item.ID == id {
			items = append(items[:idx], items[idx+1:]...)
			*list = items
			return true
		}
		if len(item.Children) > 0 {
			if removeBulletFromList(&items[idx].Children, id) {
				*list = items
				return true
			}
		}
	}
	return false
}

func sectionOrderKey(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func compareSections(a, b Section, index map[string]int) bool {
	ai := sectionOrderIndex(a, index)
	bi := sectionOrderIndex(b, index)
	if ai == bi {
		return strings.ToLower(strings.TrimSpace(a.Title)) < strings.ToLower(strings.TrimSpace(b.Title))
	}
	return ai < bi
}

func sectionOrderIndex(sec Section, index map[string]int) int {
	key := sectionOrderKey(sec.ID)
	if pos, ok := index[key]; ok {
		return pos
	}
	return len(index) * 2
}
