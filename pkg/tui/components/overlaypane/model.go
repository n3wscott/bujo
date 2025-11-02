package overlaypane

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"tableflip.dev/bujo/pkg/tui/components/command"
	overlaymgr "tableflip.dev/bujo/pkg/tui/ui/overlay"
)

type focusable interface {
	Focus() tea.Cmd
}

type blurrable interface {
	Blur() tea.Cmd
}

// Model composes a background surface with an optional overlay.
type Model struct {
	width  int
	height int

	background string
	bgCursor   *tea.Cursor

	overlay   command.Overlay
	placement command.OverlayPlacement
}

// New constructs a container sized to width x height.
func New(width, height int) *Model {
	m := &Model{}
	m.SetSize(width, height)
	return m
}

// Init implements tea.Model for embedding convenience.
func (m *Model) Init() tea.Cmd { return nil }

// SetSize updates the container bounds.
func (m *Model) SetSize(width, height int) {
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}
	m.width = width
	m.height = height
	if m.overlay != nil {
		ow, oh := m.overlaySize()
		m.overlay.SetSize(ow, oh)
	}
}

// SetBackground records the background view and cursor.
func (m *Model) SetBackground(view string, cursor *tea.Cursor) {
	m.background = view
	if cursor != nil {
		copy := *cursor
		m.bgCursor = &copy
	} else {
		m.bgCursor = nil
	}
}

// SetOverlay mounts an overlay using the provided placement.
func (m *Model) SetOverlay(overlay command.Overlay, placement command.OverlayPlacement) tea.Cmd {
	if overlay == nil {
		return nil
	}
	m.overlay = overlay
	m.placement = placement
	ow, oh := m.overlaySize()
	m.overlay.SetSize(ow, oh)
	return m.overlay.Init()
}

// ClearOverlay removes any active overlay.
func (m *Model) ClearOverlay() {
	m.overlay = nil
}

// HasOverlay reports if an overlay is currently mounted.
func (m *Model) HasOverlay() bool { return m.overlay != nil }

// Update forwards messages to the overlay when present.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	if m.overlay == nil {
		return nil
	}
	next, cmd := m.overlay.Update(msg)
	if next == nil {
		m.overlay = nil
	} else {
		m.overlay = next
	}
	return cmd
}

// View renders the composed view.
func (m *Model) View() (string, *tea.Cursor) {
	placement := m.composePlacement()
	view := overlaymgr.Compose(m.background, m.width, m.height, m.overlayView(), placement)
	cursor := m.combinedCursor()
	return view, cursor
}

// Focus requests focus for the overlay when supported.
func (m *Model) Focus() tea.Cmd {
	if f, ok := m.overlay.(focusable); ok {
		return f.Focus()
	}
	return nil
}

// Blur requests blur for the overlay when supported.
func (m *Model) Blur() tea.Cmd {
	if b, ok := m.overlay.(blurrable); ok {
		return b.Blur()
	}
	return nil
}

func (m *Model) overlayView() string {
	if m.overlay == nil {
		return ""
	}
	view, _ := m.overlay.View()
	return view
}

func (m *Model) combinedCursor() *tea.Cursor {
	if m.overlay != nil {
		_, cur := m.overlay.View()
		if cur != nil {
			offsetX, offsetY := m.computeOffsets()
			copy := *cur
			copy.X += offsetX
			copy.Y += offsetY
			return &copy
		}
	}
	if m.bgCursor != nil {
		copy := *m.bgCursor
		return &copy
	}
	return nil
}

func (m *Model) overlaySize() (int, int) {
	w := m.placement.Width
	if w <= 0 || w > m.width {
		w = m.width
	}
	h := m.placement.Height
	if h <= 0 || h > m.height {
		h = m.height
	}
	return w, h
}

func (m *Model) composePlacement() overlaymgr.Placement {
	if m.placement.Fullscreen {
		return overlaymgr.Placement{
			Horizontal: lipgloss.Left,
			Vertical:   lipgloss.Top,
			MarginX:    0,
			MarginY:    0,
			Width:      m.width,
			Height:     m.height,
		}
	}
	w, h := m.overlaySize()
	return overlaymgr.Placement{
		Horizontal: m.placement.Horizontal,
		Vertical:   m.placement.Vertical,
		MarginX:    m.placement.MarginX,
		MarginY:    m.placement.MarginY,
		Width:      w,
		Height:     h,
	}
}

func (m *Model) computeOffsets() (int, int) {
	h := m.placement.Horizontal
	if h == 0 {
		h = lipgloss.Center
	}
	v := m.placement.Vertical
	if v == 0 {
		v = lipgloss.Center
	}
	offsetX := m.placement.MarginX
	ow, oh := m.overlaySize()
	switch h {
	case lipgloss.Right:
		offsetX = m.width - ow - m.placement.MarginX
	case lipgloss.Center:
		offsetX = (m.width - ow) / 2
	}
	if offsetX < 0 {
		offsetX = 0
	}
	if offsetX > m.width-ow {
		offsetX = m.width - ow
	}
	offsetY := m.placement.MarginY
	switch v {
	case lipgloss.Bottom:
		offsetY = m.height - oh - m.placement.MarginY
	case lipgloss.Center:
		offsetY = (m.height - oh) / 2
	}
	if offsetY < 0 {
		offsetY = 0
	}
	if offsetY > m.height-oh {
		offsetY = m.height - oh
	}
	return offsetX, offsetY
}
