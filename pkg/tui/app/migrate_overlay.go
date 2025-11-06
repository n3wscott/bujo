package app

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	bulletdetail "tableflip.dev/bujo/pkg/tui/components/bulletdetail"
	collectiondetail "tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	collectionnav "tableflip.dev/bujo/pkg/tui/components/collectionnav"
	"tableflip.dev/bujo/pkg/tui/components/command"
	"tableflip.dev/bujo/pkg/tui/events"
)

func shouldForwardDetailMsg(msg tea.Msg) bool {
	switch msg.(type) {
	case tea.KeyMsg, tea.WindowSizeMsg, events.FocusMsg, events.BlurMsg:
		return true
	default:
		return false
	}
}

type migrationFocus int

const (
	migrationFocusDetail migrationFocus = iota
	migrationFocusFutureNav
	migrationFocusTargetNav
)

const (
	migrateDetailID     = events.ComponentID("MigrateDetail")
	migrateFutureNavID  = events.ComponentID("MigrateFutureNav")
	migrateTargetNavID  = events.ComponentID("MigrateTargetNav")
	migrateOverlayTitle = "Migration candidates"
)

const (
	migrateHeaderTimeFormat = "2006-01-02 15:04"
)

type migrationCreateCollectionMsg struct {
	Name string
}

type migrationCreateCollectionCancelledMsg struct{}

type migrationOverlay struct {
	data      *migrationData
	list      *collectiondetail.Model
	detail    *bulletdetail.Model
	futureNav *collectionnav.Model
	targetNav *collectionnav.Model
	window    migrationWindow
	focus     migrationFocus
	logger    io.Writer

	width        int
	height       int
	contentH     int
	leftWidth    int
	rightWidth   int
	centerWidth  int
	topHeight    int
	bottomHeight int

	futureSelection events.CollectionRef
	targetSelection events.CollectionRef

	creatingNew   bool
	createInput   textinput.Model
	createConfirm bool
	createPending string
	createError   string
}

func newMigrationOverlay(data *migrationData, window migrationWindow, futureNav, targetNav *collectionnav.Model, logger io.Writer) *migrationOverlay {
	list := collectiondetail.NewModel(nil)
	list.SetID(migrateDetailID)
	list.SetSections(data.Sections())
	detail := bulletdetail.New("", "", "", "")
	detail.SetLoading(false)
	if futureNav != nil {
		futureNav.SetID(migrateFutureNavID)
	}
	if targetNav != nil {
		targetNav.SetID(migrateTargetNavID)
	}
	input := textinput.New()
	input.Placeholder = "New collection name"
	input.CharLimit = 256
	input.Prompt = "> "
	overlay := &migrationOverlay{
		data:        data,
		list:        list,
		detail:      detail,
		futureNav:   futureNav,
		targetNav:   targetNav,
		window:      window,
		focus:       migrationFocusDetail,
		logger:      logger,
		createInput: input,
	}
	overlay.initializeDetail()
	return overlay
}

func (o *migrationOverlay) Init() tea.Cmd {
	var cmds []tea.Cmd
	if o.list != nil {
		if cmd := o.list.Focus(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if o.futureNav != nil {
		if cmd := o.futureNav.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if o.targetNav != nil {
		if cmd := o.targetNav.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (o *migrationOverlay) Update(msg tea.Msg) (command.Overlay, tea.Cmd) {
	if o.creatingNew {
		return o.updateCreateNewCollection(msg)
	}
	var (
		cmds             []tea.Cmd
		skipDetailUpdate bool
	)
	switch v := msg.(type) {
	case tea.KeyMsg:
		switch v.String() {
		case "<":
			if o.futureNav != nil {
				o.switchFocus(migrationFocusFutureNav, &cmds)
				skipDetailUpdate = true
			}
		case ">":
			if o.targetNav != nil {
				o.switchFocus(migrationFocusTargetNav, &cmds)
				skipDetailUpdate = true
			}
		}
	}

	switch msg.(type) {
	case events.CollectionChangeMsg, events.CollectionOrderMsg:
		if len(cmds) == 0 {
			return o, nil
		}
		return o, tea.Batch(cmds...)
	}

	if o.futureNav != nil {
		if next, cmd := o.futureNav.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		} else {
			o.futureNav = next.(*collectionnav.Model)
		}
	}
	if o.targetNav != nil {
		if next, cmd := o.targetNav.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		} else {
			o.targetNav = next.(*collectionnav.Model)
		}
	}
	if o.list != nil && !skipDetailUpdate && shouldForwardDetailMsg(msg) {
		if next, cmd := o.list.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		} else if nextModel, ok := next.(*collectiondetail.Model); ok {
			o.list = nextModel
		}
	}
	if o.detail != nil {
		if next, cmd := o.detail.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		} else if d, ok := next.(*bulletdetail.Model); ok {
			o.detail = d
		}
	}

	switch v := msg.(type) {
	case events.CollectionHighlightMsg:
		o.handleCollectionHighlight(v)
	case events.BulletHighlightMsg:
		if cmd := o.handleBulletHighlight(v); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if len(cmds) == 0 {
		return o, nil
	}
	return o, tea.Batch(cmds...)
}

func (o *migrationOverlay) updateCreateNewCollection(msg tea.Msg) (command.Overlay, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyMsg:
		switch v.String() {
		case "enter":
			name := strings.TrimSpace(o.createInput.Value())
			if name == "" {
				o.createError = "Collection name cannot be empty"
				return o, nil
			}
			if !o.createConfirm {
				o.createConfirm = true
				o.createPending = name
				o.createError = ""
				return o, nil
			}
			if o.createPending != name {
				// Value changed after confirmation; reset confirmation state.
				o.createConfirm = false
				o.createPending = ""
				o.createError = ""
				return o, nil
			}
			finalName := name
			o.creatingNew = false
			o.createConfirm = false
			o.createPending = ""
			o.createError = ""
			o.createInput.Blur()
			return o, func() tea.Msg {
				return migrationCreateCollectionMsg{Name: finalName}
			}
		case "esc":
			if o.createConfirm {
				o.createConfirm = false
				o.createPending = ""
				o.createError = ""
				return o, nil
			}
			o.creatingNew = false
			o.createConfirm = false
			o.createPending = ""
			o.createError = ""
			o.createInput.Blur()
			o.createInput.SetValue("")
			return o, func() tea.Msg { return migrationCreateCollectionCancelledMsg{} }
		default:
			o.createError = ""
		}
	}
	model, cmd := o.createInput.Update(msg)
	o.createInput = model
	if o.createConfirm && strings.TrimSpace(o.createInput.Value()) != o.createPending {
		o.createConfirm = false
		o.createPending = ""
	}
	if _, ok := msg.(tea.KeyMsg); !ok {
		o.createError = ""
	}
	return o, cmd
}

func (o *migrationOverlay) BeginNewCollectionPrompt() tea.Cmd {
	o.creatingNew = true
	o.createConfirm = false
	o.createPending = ""
	o.createError = ""
	o.createInput.SetValue("")
	var cmds []tea.Cmd
	o.switchFocus(migrationFocusDetail, &cmds)
	if cmd := o.createInput.Focus(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (o *migrationOverlay) handleCollectionHighlight(msg events.CollectionHighlightMsg) {
	switch msg.Component {
	case migrateFutureNavID:
		o.futureSelection = msg.Collection
	case migrateTargetNavID:
		o.targetSelection = msg.Collection
	}
}

func (o *migrationOverlay) handleBulletHighlight(msg events.BulletHighlightMsg) tea.Cmd {
	if msg.Component != migrateDetailID {
		return nil
	}
	if o.data == nil {
		return nil
	}
	bulletID := strings.TrimSpace(msg.Bullet.ID)
	item, ok := o.data.Bullet(bulletID)
	if !ok {
		return nil
	}
	o.updateDetailView(item, msg.Collection.Title, msg.Bullet.Label)
	if o.futureNav != nil && isFutureCollection(item.SectionID) {
		if cmd := o.futureNav.SelectCollection(item.CollectionRef); cmd != nil {
			return cmd
		}
	} else if o.targetNav != nil {
		if cmd := o.targetNav.SelectCollection(item.CollectionRef); cmd != nil {
			return cmd
		}
	}
	return nil
}

func (o *migrationOverlay) updateDetailView(item *migrationBullet, collectionTitle, bulletLabel string) {
	if item == nil || item.Candidate.Entry == nil {
		return
	}
	parentLabel := item.ParentLabel
	if o.detail == nil {
		o.detail = bulletdetail.New(collectionTitle, bulletLabel, item.SectionID, parentLabel)
	} else {
		o.detail = bulletdetail.New(collectionTitle, bulletLabel, item.SectionID, parentLabel)
	}
	o.detail.SetSize(o.centerWidth, o.bottomHeight)
	o.detail.SetEntry(item.Candidate.Entry)
}

func (o *migrationOverlay) View() (string, *tea.Cursor) {
	header := o.renderHeader()
	left := o.renderNav(o.futureNav, o.leftWidth, o.contentH)
	right := o.renderNav(o.targetNav, o.rightWidth, o.contentH)
	center := o.renderCenter()

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, verticalDivider(o.contentH), center, verticalDivider(o.contentH), right)
	content := lipgloss.JoinVertical(lipgloss.Left, header, body)
	return lipgloss.NewStyle().Width(o.width).Height(o.height).Render(content), nil
}

func (o *migrationOverlay) renderHeader() string {
	label := migrateOverlayTitle
	windowLabel := strings.TrimSpace(o.window.Label)
	if windowLabel == "" {
		windowLabel = "all open tasks"
	}
	header := fmt.Sprintf("%s · %s", label, windowLabel)
	if span := formatMigrationWindowSpan(o.window); span != "" {
		header = fmt.Sprintf("%s %s", header, span)
	}
	return lipgloss.NewStyle().Bold(true).Width(o.width).Render(header)
}

func (o *migrationOverlay) renderNav(nav *collectionnav.Model, width, height int) string {
	if nav == nil || width <= 0 || height <= 0 {
		return lipgloss.NewStyle().Width(width).Height(height).Render("")
	}
	view := nav.View()
	if strings.Contains(view, migrationNewCollectionID) {
		view = strings.ReplaceAll(view, migrationNewCollectionID, migrationNewCollectionLabel)
	}
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		AlignVertical(lipgloss.Top).
		Render(view)
}

func (o *migrationOverlay) renderCenter() string {
	var top string
	if o.list != nil {
		view := o.list.View()
		top = lipgloss.NewStyle().Width(o.centerWidth).Height(o.topHeight).Render(view)
	} else {
		top = lipgloss.NewStyle().Width(o.centerWidth).Height(o.topHeight).Render("No entries to migrate.")
	}
	var bottom string
	if o.creatingNew {
		inputView := o.createInput.View()
		info := "Press Enter to continue, Esc to cancel."
		if o.createConfirm && o.createPending != "" {
			info = fmt.Sprintf("Press Enter again to create %q, Esc to edit.", o.createPending)
		}
		if strings.TrimSpace(o.createInput.Value()) == "" {
			info = "Enter a new collection name. Esc cancels."
		}
		message := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(info)
		if strings.TrimSpace(o.createError) != "" {
			message = lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Render(o.createError)
		}
		prompt := lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render("Create New Collection"),
			inputView,
			message,
		)
		bottom = lipgloss.NewStyle().
			Width(o.centerWidth).
			Height(o.bottomHeight).
			Align(lipgloss.Left, lipgloss.Top).
			Render(prompt)
	} else if o.detail != nil {
		view, _ := o.detail.View()
		bottom = lipgloss.NewStyle().Width(o.centerWidth).Height(o.bottomHeight).Render(view)
	} else {
		bottom = lipgloss.NewStyle().Width(o.centerWidth).Height(o.bottomHeight).Render("")
	}
	divider := horizontalDivider(o.centerWidth)
	return lipgloss.JoinVertical(lipgloss.Top, top, divider, bottom)
}

func (o *migrationOverlay) SetSize(width, height int) {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	o.width = width
	o.height = height
	headerRows := 2
	contentHeight := height - headerRows
	if contentHeight <= 0 {
		contentHeight = 1
	}
	o.contentH = contentHeight

	const (
		preferredNavWidth = 24
		minNavWidth       = 14
		minCenterWidth    = 30
	)

	remaining := width - 2
	left := preferredNavWidth
	right := preferredNavWidth
	center := remaining - left - right
	if center < minCenterWidth {
		deficit := minCenterWidth - center
		reclaim := (deficit + 1) / 2
		left = maxInt(minNavWidth, left-reclaim)
		right = maxInt(minNavWidth, right-reclaim)
		center = remaining - left - right
		if center < minCenterWidth {
			center = minCenterWidth
			if leftover := remaining - center; leftover > 0 {
				left = leftover / 2
				right = leftover - left
			}
		}
	}
	if center <= 0 {
		center = minCenterWidth
	}
	o.leftWidth = maxInt(0, left)
	o.rightWidth = maxInt(0, right)
	o.centerWidth = maxInt(20, center)

	available := contentHeight - 1
	if available <= 0 {
		available = contentHeight
	}
	top := available / 2
	bottom := available - top
	if top < 6 {
		top = 6
		bottom = available - top
	}
	if bottom < 6 {
		bottom = 6
		top = available - bottom
		if top < 4 {
			top = 4
		}
	}
	o.topHeight = top
	o.bottomHeight = bottom

	if o.futureNav != nil {
		o.futureNav.SetSize(o.leftWidth, contentHeight)
	}
	if o.targetNav != nil {
		o.targetNav.SetSize(o.rightWidth, contentHeight)
	}
	if o.list != nil {
		o.list.SetSize(o.centerWidth, o.topHeight)
	}
	if o.detail != nil {
		o.detail.SetSize(o.centerWidth, o.bottomHeight)
	}
}

func (o *migrationOverlay) switchFocus(next migrationFocus, cmds *[]tea.Cmd) {
	if next == o.focus {
		return
	}
	switch o.focus {
	case migrationFocusDetail:
		if o.list != nil {
			if cmd := o.list.Blur(); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
		}
	case migrationFocusFutureNav:
		if o.futureNav != nil {
			if cmd := o.futureNav.Blur(); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
		}
	case migrationFocusTargetNav:
		if o.targetNav != nil {
			if cmd := o.targetNav.Blur(); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
		}
	}
	o.focus = next
	switch o.focus {
	case migrationFocusDetail:
		if o.list != nil {
			if cmd := o.list.Focus(); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
		}
	case migrationFocusFutureNav:
		if o.futureNav != nil {
			if cmd := o.futureNav.Focus(); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
		}
	case migrationFocusTargetNav:
		if o.targetNav != nil {
			if cmd := o.targetNav.Focus(); cmd != nil {
				*cmds = append(*cmds, cmd)
			}
		}
	}
}

func (o *migrationOverlay) SetData(data *migrationData) {
	o.data = data
	if o.list != nil {
		if data == nil {
			o.list.SetSections(nil)
		} else {
			o.list.SetSections(data.Sections())
		}
	}
	if data == nil && o.detail != nil {
		o.detail = bulletdetail.New("", "", "", "")
		o.detail.SetSize(o.centerWidth, o.bottomHeight)
		o.detail.SetEntry(nil)
	}
	o.initializeDetail()
}

func (o *migrationOverlay) RemoveBullet(id string) {
	if o.data == nil {
		return
	}
	if !o.data.Remove(id) {
		return
	}
	if o.list != nil {
		o.list.SetSections(o.data.Sections())
	}
	if o.data.IsEmpty() {
		o.detail = bulletdetail.New("", "", "", "")
		o.detail.SetSize(o.centerWidth, o.bottomHeight)
		o.detail.SetEntry(nil)
	}
	o.initializeDetail()
}

func (o *migrationOverlay) IsEmpty() bool {
	if o.data == nil {
		return true
	}
	return o.data.IsEmpty()
}

func (o *migrationOverlay) CurrentBullet() (collectiondetail.Section, collectiondetail.Bullet, bool) {
	if o.list == nil {
		return collectiondetail.Section{}, collectiondetail.Bullet{}, false
	}
	return o.list.CurrentSelection()
}

func (o *migrationOverlay) FutureSelection() (events.CollectionRef, bool, bool) {
	if o.futureNav == nil {
		return events.CollectionRef{}, false, false
	}
	return o.futureNav.CurrentSelection()
}

func (o *migrationOverlay) TargetSelection() (events.CollectionRef, bool, bool) {
	if o.targetNav == nil {
		return events.CollectionRef{}, false, false
	}
	return o.targetNav.CurrentSelection()
}

func (o *migrationOverlay) CurrentMigrationSelection() (collectiondetail.Section, collectiondetail.Bullet, *migrationBullet, bool) {
	if o.list == nil || o.data == nil {
		return collectiondetail.Section{}, collectiondetail.Bullet{}, nil, false
	}
	section, bullet, ok := o.list.CurrentSelection()
	if !ok || strings.TrimSpace(bullet.ID) == "" {
		return section, bullet, nil, false
	}
	item, found := o.data.Bullet(bullet.ID)
	if !found {
		return section, bullet, nil, false
	}
	return section, bullet, item, true
}

func (o *migrationOverlay) Focus() tea.Cmd {
	if o.creatingNew {
		return o.createInput.Focus()
	}
	switch o.focus {
	case migrationFocusFutureNav:
		if o.futureNav != nil {
			return o.futureNav.Focus()
		}
	case migrationFocusTargetNav:
		if o.targetNav != nil {
			return o.targetNav.Focus()
		}
	default:
		if o.list != nil {
			return o.list.Focus()
		}
	}
	return nil
}

func (o *migrationOverlay) Blur() tea.Cmd {
	var cmds []tea.Cmd
	if o.creatingNew {
		o.createInput.Blur()
	}
	if o.list != nil {
		if cmd := o.list.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if o.futureNav != nil {
		if cmd := o.futureNav.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if o.targetNav != nil {
		if cmd := o.targetNav.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (o *migrationOverlay) FocusDetail() tea.Cmd {
	if o.list == nil {
		return nil
	}
	var cmds []tea.Cmd
	if o.creatingNew {
		o.creatingNew = false
		o.createConfirm = false
		o.createPending = ""
		o.createError = ""
		o.createInput.Blur()
		o.createInput.SetValue("")
	}
	if o.futureNav != nil {
		if cmd := o.futureNav.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if o.targetNav != nil {
		if cmd := o.targetNav.Blur(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if cmd := o.list.Focus(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	o.focus = migrationFocusDetail
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (o *migrationOverlay) initializeDetail() {
	if o.list == nil || o.data == nil {
		return
	}
	section, bullet, ok := o.list.CurrentSelection()
	if !ok || strings.TrimSpace(bullet.ID) == "" {
		return
	}
	item, exists := o.data.Bullet(bullet.ID)
	if !exists {
		return
	}
	o.updateDetailView(item, section.Title, bullet.Label)
	if o.futureNav != nil && isFutureCollection(item.SectionID) {
		_ = o.futureNav.SelectCollection(item.CollectionRef)
	} else if o.targetNav != nil {
		_ = o.targetNav.SelectCollection(item.CollectionRef)
	}
}

func horizontalDivider(width int) string {
	if width <= 0 {
		width = 1
	}
	line := strings.Repeat("─", width)
	return lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(line)
}

func formatMigrationWindowSpan(window migrationWindow) string {
	since := window.Since
	until := window.Until
	if window.HasWindow {
		if until.IsZero() {
			until = time.Now()
		}
		if since.IsZero() && window.Duration > 0 {
			since = until.Add(-window.Duration)
		}
	}
	if since.IsZero() || until.IsZero() {
		return ""
	}
	return fmt.Sprintf("(%s → %s)", since.Local().Format(migrateHeaderTimeFormat), until.Local().Format(migrateHeaderTimeFormat))
}
