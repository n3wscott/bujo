package timeutil

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultWindow is the fallback report window used when none is provided.
	DefaultWindow = "1w"
)

var (
	windowPattern = regexp.MustCompile(`^\s*(\d+)\s*([a-z]+)`)
	unitMap       = map[string]time.Duration{
		"s":       time.Second,
		"sec":     time.Second,
		"secs":    time.Second,
		"second":  time.Second,
		"seconds": time.Second,
		"m":       time.Minute,
		"min":     time.Minute,
		"mins":    time.Minute,
		"minute":  time.Minute,
		"minutes": time.Minute,
		"h":       time.Hour,
		"hr":      time.Hour,
		"hrs":     time.Hour,
		"hour":    time.Hour,
		"hours":   time.Hour,
		"d":       24 * time.Hour,
		"day":     24 * time.Hour,
		"days":    24 * time.Hour,
		"w":       7 * 24 * time.Hour,
		"wk":      7 * 24 * time.Hour,
		"wks":     7 * 24 * time.Hour,
		"week":    7 * 24 * time.Hour,
		"weeks":   7 * 24 * time.Hour,
	}
)

// ParseWindow parses a human-friendly duration string (for example "1w", "3d", or
// "1w2d6h") and returns the equivalent duration along with a canonical, compact
// representation. When the input is empty, the default window of one week is used.
func ParseWindow(input string) (time.Duration, string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		trimmed = DefaultWindow
	}

	lower := strings.ToLower(trimmed)
	remaining := lower
	total := time.Duration(0)
	for len(remaining) > 0 {
		matches := windowPattern.FindStringSubmatch(remaining)
		if len(matches) != 3 {
			return 0, "", fmt.Errorf("invalid duration segment %q", strings.TrimSpace(remaining))
		}
		valueStr := matches[1]
		unitStr := matches[2]

		value, err := strconv.ParseInt(valueStr, 10, 64)
		if err != nil {
			return 0, "", fmt.Errorf("invalid duration value %q: %w", valueStr, err)
		}
		base, ok := unitMap[unitStr]
		if !ok {
			return 0, "", fmt.Errorf("unsupported duration unit %q", unitStr)
		}
		total += time.Duration(value) * base

		remaining = remaining[len(matches[0]):]
	}

	if total <= 0 {
		return 0, "", fmt.Errorf("duration must be greater than zero")
	}

	return total, FormatWindow(total), nil
}

// FormatWindow renders a duration using week/day/hour/minute/second tokens.
func FormatWindow(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}

	type unit struct {
		label string
		value time.Duration
	}
	units := []unit{
		{"w", 7 * 24 * time.Hour},
		{"d", 24 * time.Hour},
		{"h", time.Hour},
		{"m", time.Minute},
		{"s", time.Second},
	}

	var parts []string
	remaining := d
	for _, u := range units {
		if remaining < u.value {
			continue
		}
		count := remaining / u.value
		remaining -= count * u.value
		parts = append(parts, fmt.Sprintf("%d%s", count, u.label))
	}
	if len(parts) == 0 {
		return "0s"
	}
	return strings.Join(parts, "")
}
