package theme

import "github.com/charmbracelet/lipgloss/v2"

// Theme centralizes Lip Gloss styles for the Bubble Tea UI.
type Theme struct {
	Footer FooterTheme
}

// FooterTheme groups styles used by the bottom status/command bar.
type FooterTheme struct {
	Help                lipgloss.Style
	Status              lipgloss.Style
	Bullet              lipgloss.Style
	CommandName         lipgloss.Style
	CommandDescription  lipgloss.Style
	CommandSelectedName lipgloss.Style
	CommandSelectedDesc lipgloss.Style
}

// Default returns the built-in theme used across the UI.
func Default() Theme {
	commandName := lipgloss.NewStyle().
		Foreground(lipgloss.Color("212")).
		Bold(true)
	commandDesc := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	return Theme{
		Footer: FooterTheme{
			Help:               lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
			Status:             lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
			Bullet:             lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
			CommandName:        commandName,
			CommandDescription: commandDesc,
			CommandSelectedName: commandName.
				Copy().
				Reverse(true),
			CommandSelectedDesc: commandDesc.
				Copy().
				Reverse(true),
		},
	}
}
