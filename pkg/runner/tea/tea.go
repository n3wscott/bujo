package teaui

import (
	appsvc "tableflip.dev/bujo/pkg/app"
	tuiapp "tableflip.dev/bujo/pkg/tui/app"
)

// Run launches the Bubble Tea UI (legacy entrypoint wrapping pkg/tui/app).
func Run(svc *appsvc.Service) error {
	return tuiapp.Run(svc)
}
