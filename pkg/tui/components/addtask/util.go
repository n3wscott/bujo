package addtask

import "strings"

import "tableflip.dev/bujo/pkg/glyph"

func clampIndex(value, length int) int {
	if length <= 0 {
		return 0
	}
	if value < 0 {
		return 0
	}
	if value >= length {
		return length - 1
	}
	return value
}

func collectionLabel(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "(unnamed)"
	}
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}

func describePrompt(b glyph.Bullet, s glyph.Signifier, parent string) string {
	base := "Describe the task..."
	parent = strings.ToLower(parent)
	switch b {
	case glyph.Event:
		base = "Describe the event..."
	case glyph.Note:
		base = "Describe the note..."
	}
	if strings.Contains(parent, "event") {
		base = strings.Replace(base, "task", "event", 1)
	}
	if strings.Contains(parent, "note") {
		base = strings.Replace(base, "task", "note", 1)
	}
	switch s {
	case glyph.Priority:
		return strings.Replace(base, "Describe", "Describe the important", 1)
	case glyph.Inspiration:
		return strings.Replace(base, "Describe", "Describe the inspiration", 1)
	case glyph.Investigation:
		return strings.Replace(base, "Describe", "Describe the investigation", 1)
	default:
		return base
	}
}
