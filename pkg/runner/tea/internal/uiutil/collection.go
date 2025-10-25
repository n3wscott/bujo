package uiutil

import (
	"fmt"
	"strings"
	"time"
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
