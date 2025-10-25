package ui

import tea "github.com/charmbracelet/bubbletea/v2"

// Component defines the contract for reusable Bubble Tea widgets.
type Component interface {
	Init() tea.Cmd
	Update(tea.Msg) (Component, tea.Cmd)
	View() string
	SetSize(width, height int)
}
