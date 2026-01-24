package clock

import "time"

// Clock supplies time for UI ordering and tests.
type Clock interface {
	Now() time.Time
}

// RealClock uses the system clock.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

// FixedClock returns a stable time for deterministic tests.
type FixedClock struct {
	Fixed time.Time
}

func (c FixedClock) Now() time.Time { return c.Fixed }
