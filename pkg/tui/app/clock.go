package app

import (
	"time"

	"tableflip.dev/bujo/pkg/tui/clock"
)

func (m *Model) now() time.Time {
	if m.clock != nil {
		return m.clock.Now()
	}
	return clock.RealClock{}.Now()
}
