package teaui

import (
	appsvc "tableflip.dev/bujo/pkg/app"
	tuiapp "tableflip.dev/bujo/pkg/tui/app"
)

// Run launches the Bubble Tea UI (new Bubble Tea surface).
func Run(svc *appsvc.Service) error {
	return tuiapp.Run(svc)
}
