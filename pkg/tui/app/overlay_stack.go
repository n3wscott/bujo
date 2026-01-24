package app

import (
	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/tui/components/command"
	overlaypane "tableflip.dev/bujo/pkg/tui/components/overlaypane"
)

// overlayStack manages the active overlay and synchronizes it with the overlay pane renderer.
type overlayStack struct {
	pane   *overlaypane.Model
	active overlayKind
}

func newOverlayStack(width, height int) *overlayStack {
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}
	return &overlayStack{
		pane: overlaypane.New(width, height),
	}
}

func (s *overlayStack) ActiveKind() overlayKind {
	return s.active
}

func (s *overlayStack) HasOverlay() bool {
	return s.pane != nil && s.pane.HasOverlay()
}

func (s *overlayStack) SetSize(width, height int) {
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}
	if s.pane == nil {
		s.pane = overlaypane.New(width, height)
		return
	}
	s.pane.SetSize(width, height)
}

func (s *overlayStack) SetBackground(view string, cursor *tea.Cursor) {
	if s.pane == nil {
		s.pane = overlaypane.New(1, 1)
	}
	s.pane.SetBackground(view, cursor)
}

func (s *overlayStack) View() (string, *tea.Cursor) {
	if s.pane == nil {
		return "", nil
	}
	return s.pane.View()
}

func (s *overlayStack) Open(kind overlayKind, overlay command.Overlay, placement command.OverlayPlacement) tea.Cmd {
	if overlay == nil {
		return nil
	}
	if s.pane == nil {
		s.pane = overlaypane.New(1, 1)
	}
	s.active = kind
	return s.pane.SetOverlay(overlay, placement)
}

func (s *overlayStack) Close(kind overlayKind) {
	if s.active != kind {
		return
	}
	s.active = overlayKindNone
	if s.pane != nil {
		s.pane.ClearOverlay()
	}
}

func (s *overlayStack) Dismiss() {
	if s.active == overlayKindNone {
		return
	}
	s.active = overlayKindNone
	if s.pane != nil {
		s.pane.ClearOverlay()
	}
}

func (s *overlayStack) Update(msg tea.Msg) (overlayKind, tea.Cmd) {
	if s.pane == nil {
		return overlayKindNone, nil
	}
	cmd := s.pane.Update(msg)
	if s.active != overlayKindNone && !s.pane.HasOverlay() {
		closed := s.active
		s.active = overlayKindNone
		return closed, cmd
	}
	return overlayKindNone, cmd
}

func (s *overlayStack) Focus() tea.Cmd {
	if s.pane == nil || !s.pane.HasOverlay() {
		return nil
	}
	return s.pane.Focus()
}

func (s *overlayStack) Blur() tea.Cmd {
	if s.pane == nil || !s.pane.HasOverlay() {
		return nil
	}
	return s.pane.Blur()
}
