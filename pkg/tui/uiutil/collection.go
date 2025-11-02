package uiutil

import (
	"fmt"
	"strings"
	"time"

	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/tui/components/index"
)

// FriendlyCollectionName renders a human-readable label for collection IDs.
func FriendlyCollectionName(id string) string {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "/") {
		parts := strings.Split(trimmed, "/")
		if len(parts) == 2 {
			child := strings.TrimSpace(parts[1])
			if day, err := time.Parse("January 2, 2006", child); err == nil {
				return day.Format("Monday, January 2, 2006")
			}
			if month, err := time.Parse("January 2006", child); err == nil {
				return month.Format("January, 2006")
			}
			return child
		}
	}
	if day, err := time.Parse("January 2, 2006", trimmed); err == nil {
		return day.Format("Monday, January 2, 2006")
	}
	if month, err := time.Parse("January 2006", trimmed); err == nil {
		return month.Format("January, 2006")
	}
	return trimmed
}

// FormattedCollectionName includes parent segments for display.
func FormattedCollectionName(id string) string {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "/") {
		parts := strings.Split(trimmed, "/")
		if len(parts) == 2 {
			parent := strings.TrimSpace(parts[0])
			child := strings.TrimSpace(parts[1])
			if _, err := time.Parse("January 2, 2006", child); err == nil {
				if day, err := time.Parse("January 2, 2006", child); err == nil {
					return day.Format("Monday, January 2, 2006")
				}
			}
			if _, err := time.Parse("January 2006", child); err == nil {
				if month, err := time.Parse("January 2006", child); err == nil {
					return fmt.Sprintf("%s › %s", parent, month.Format("January, 2006"))
				}
			}
			return fmt.Sprintf("%s › %s", parent, child)
		}
	}
	return FriendlyCollectionName(trimmed)
}

// FormatReportTime renders a compact timestamp for report headers.
func FormatReportTime(t time.Time) string {
	if t.IsZero() {
		return "(unknown)"
	}
	return t.Local().Format("2006-01-02 15:04")
}

// EntryLabel returns a user-friendly label for an entry.
func EntryLabel(e *entry.Entry) string {
	if e == nil {
		return "<unknown>"
	}
	msg := strings.TrimSpace(e.Message)
	if msg != "" {
		return msg
	}
	if e.Collection != "" {
		return e.Collection
	}
	if e.ID != "" {
		return e.ID
	}
	return "<entry>"
}

// LastSegment returns the trailing path component after '/'.
func LastSegment(path string) string {
	trimmed := strings.TrimSpace(path)
	if idx := strings.LastIndex(trimmed, "/"); idx >= 0 {
		return trimmed[idx+1:]
	}
	return trimmed
}

// ParentCollectionName returns the parent portion of "parent/child".
func ParentCollectionName(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if idx := strings.LastIndex(trimmed, "/"); idx > 0 {
		return trimmed[:idx]
	}
	return ""
}

// ParseDayNumber best-effort extracts a day number from a collection path.

// ParseDayNumber extracts the numerical day for the parent/month context.
func ParseDayNumber(parent, child string) int {
	if t := ParseDay(parent, child); !t.IsZero() {
		return t.Day()
	}
	return 0
}

// ParseDay best-effort parses a "Month/Day" style name.
func ParseDay(parent, child string) time.Time {
	monthTime, err := time.Parse("January 2006", parent)
	if err != nil {
		return time.Time{}
	}
	layout := "January 2, 2006"
	if strings.Contains(child, ",") {
		if t, err := time.Parse(layout, child); err == nil {
			return t
		}
	}
	full := fmt.Sprintf("%s %s", parent, strings.TrimSpace(strings.TrimPrefix(child, parent)))
	if t, err := time.Parse(layout, full); err == nil {
		return t
	}
	if day := index.DayNumberFromName(monthTime, child); day > 0 {
		return time.Date(monthTime.Year(), monthTime.Month(), day, 0, 0, 0, 0, time.Local)
	}
	return time.Time{}
}
