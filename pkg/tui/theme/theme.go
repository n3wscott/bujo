package theme

import "github.com/charmbracelet/lipgloss/v2"

// Theme centralizes Lip Gloss styles for the Bubble Tea UI.
type Theme struct {
	Footer   FooterTheme
	Panel    PanelTheme
	Report   ReportTheme
	Modal    ModalTheme
	Calendar CalendarTheme
	Accent   lipgloss.Style
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

// CalendarTheme styles the calendar grid used in nav/index views.
type CalendarTheme struct {
	Header   lipgloss.Style
	Empty    lipgloss.Style
	Entry    lipgloss.Style
	Today    lipgloss.Style
	Selected lipgloss.Style
}

// PanelTheme styles framed panels and headings.
type PanelTheme struct {
	Frame lipgloss.Style
	Title lipgloss.Style
	Body  lipgloss.Style
}

// ReportTheme styles the completion report overlay.
type ReportTheme struct {
	Frame  lipgloss.Style
	Header lipgloss.Style
	Text   lipgloss.Style
}

// ModalTheme styles centered modal overlays (e.g., wizard).
type ModalTheme struct {
	Frame lipgloss.Style
	Title lipgloss.Style
	Body  lipgloss.Style
}

// Default returns the built-in theme used across the UI.
func Default() Theme {
	commandName := lipgloss.NewStyle().
		Foreground(lipgloss.Color("212")).
		Bold(true)
	commandDesc := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	commandSelectedName := commandName.Reverse(true)
	commandSelectedDesc := commandDesc.Reverse(true)
	calendarHeader := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Bold(true)
	calendarEmpty := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	calendarEntry := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	calendarToday := lipgloss.NewStyle().Underline(true)
	calendarSelected := lipgloss.NewStyle().Background(lipgloss.Color("63")).Foreground(lipgloss.Color("0"))

	return Theme{
		Footer: FooterTheme{
			Help:                lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
			Status:              lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("214")),
			Bullet:              lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
			CommandName:         commandName,
			CommandDescription:  commandDesc,
			CommandSelectedName: commandSelectedName,
			CommandSelectedDesc: commandSelectedDesc,
		},
		Panel: PanelTheme{
			Frame: lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				Padding(1, 2),
			Title: lipgloss.NewStyle().Bold(true),
			Body:  lipgloss.NewStyle(),
		},
		Report: ReportTheme{
			Frame:  lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).Padding(1, 2),
			Header: lipgloss.NewStyle().Bold(true),
			Text:   lipgloss.NewStyle(),
		},
		Modal: ModalTheme{
			Frame: lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				Padding(1, 2),
			Title: lipgloss.NewStyle().Bold(true),
			Body:  lipgloss.NewStyle(),
		},
		Calendar: CalendarTheme{
			Header:   calendarHeader,
			Empty:    calendarEmpty,
			Entry:    calendarEntry,
			Today:    calendarToday,
			Selected: calendarSelected,
		},
		Accent: lipgloss.NewStyle().Foreground(lipgloss.Color("213")),
	}
}
