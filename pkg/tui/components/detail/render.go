package detail

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/muesli/reflow/wordwrap"

	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/tui/theme"
)

// Viewport renders sections within height, returning string lines and content height.
func (s *State) Viewport(height int) (string, int) {
	if height <= 0 {
		return "", 0
	}
	s.viewHeight = height
	s.ensureScrollVisible()
	content := s.renderAll()
	if s.scrollOffset < 0 {
		s.scrollOffset = 0
	}
	if s.scrollOffset >= len(content) {
		s.scrollOffset = 0
	}
	end := s.scrollOffset + height
	if end > len(content) {
		end = len(content)
	}
	view := append([]string(nil), content[s.scrollOffset:end]...)
	for len(view) < height {
		view = append(view, "")
	}
	return strings.Join(view, "\n"), len(content)
}

func (s *State) renderAll() []string {
	var lines []string
	for idx := range s.sections {
		lines = append(lines, s.renderSection(idx)...)
	}
	return lines
}

func (s *State) sectionTop(idx int) int {
	top := 0
	for i := 0; i < idx && i < len(s.sections); i++ {
		top += s.sectionHeight(i)
	}
	return top
}

func (s *State) renderSection(idx int) []string {
	if idx < 0 || idx >= len(s.sections) {
		return nil
	}
	section := s.sections[idx]
	header := formatCollectionTitle(section.CollectionName, section.ResolvedName)
	selected := idx == s.sectionIndex

	headerStyle := lipgloss.NewStyle().Bold(true)
	if selected {
		headerStyle = headerStyle.Inherit(theme.Default().Accent)
	}

	lines := []string{headerStyle.Render(header)}

	offsets := make([]int, len(section.Entries))
	heights := make([]int, len(section.Entries))
	lineOffset := 1
	if len(section.Entries) == 0 {
		lines = append(lines, "  <empty>")
	} else {
		for entryIdx, item := range section.Entries {
			if !s.isVisibleEntry(idx, entryIdx) {
				offsets[entryIdx] = -1
				heights[entryIdx] = 0
				continue
			}
			caret := " "
			if selected && entryIdx == s.entryIndex {
				caret = theme.Default().Accent.Render("→")
			}
			indent := strings.Repeat("  ", s.depthOf(idx, item.ID))
			itemLines := formatEntryLines(item, caret, indent, s.wrapWidth)
			offsets[entryIdx] = lineOffset
			heights[entryIdx] = len(itemLines)
			lines = append(lines, itemLines...)
			lineOffset += len(itemLines)
		}
	}
	lines = append(lines, "") // spacer between sections
	s.cachedHeights[idx] = len(lines)
	if len(section.Entries) == 0 {
		s.entryOffsets[idx] = nil
		s.entryHeights[idx] = nil
	} else {
		s.entryOffsets[idx] = offsets
		s.entryHeights[idx] = heights
	}
	return lines
}

func formatCollectionTitle(name, resolved string) string {
	if resolved != "" {
		if strings.Contains(resolved, "/") {
			parts := strings.SplitN(resolved, "/", 2)
			if len(parts) == 2 {
				if t, err := time.Parse("January 2, 2006", parts[1]); err == nil {
					return t.Format("Monday, January 2, 2006")
				}
				if mt, err := time.Parse("January 2006", parts[0]); err == nil {
					return mt.Format("January, 2006")
				}
			}
		}
		if t, err := time.Parse("January 2, 2006", resolved); err == nil {
			return t.Format("Monday, January 2, 2006")
		}
		if t, err := time.Parse("January 2006", resolved); err == nil {
			return t.Format("January, 2006")
		}
	}
	if t, err := time.Parse("January 2, 2006", name); err == nil {
		return t.Format("Monday, January 2, 2006")
	}
	if t, err := time.Parse("January 2006", name); err == nil {
		return t.Format("January, 2006")
	}
	return name
}

func formatEntryLines(e *entry.Entry, caret, indent string, wrapWidth int) []string {
	signifier := e.Signifier.String()
	if signifier == "" {
		signifier = " "
	}
	bulletGlyph := e.Bullet.Glyph()
	bullet := bulletGlyph.Symbol
	if bullet == "" {
		bullet = e.Bullet.String()
	}
	message := e.Message
	if strings.TrimSpace(message) == "" {
		message = "<empty>"
	}
	msgLines := strings.Split(message, "\n")
	indentStr := indent
	bulletWithIndent := bullet
	if indentStr != "" {
		bulletWithIndent = indentStr + bullet
	}
	prefix := fmt.Sprintf("%s%s %s ", caret, signifier, bulletWithIndent)
	prefixStyle := lipgloss.NewStyle()
	messageStyle := lipgloss.NewStyle()
	if e.Bullet == glyph.Completed || e.Bullet == glyph.Irrelevant {
		prefixStyle = prefixStyle.Foreground(lipgloss.Color("241"))
		messageStyle = messageStyle.Foreground(lipgloss.Color("241"))
	}
	if e.Immutable {
		prefixStyle = prefixStyle.Foreground(lipgloss.Color("244")).Faint(true)
		messageStyle = messageStyle.Foreground(lipgloss.Color("244")).Faint(true).Italic(true)
	}
	if e.Bullet == glyph.Irrelevant {
		messageStyle = messageStyle.Strikethrough(true)
	}
	width := wrapWidth
	if width <= 0 {
		width = 80
	}
	available := width - lipgloss.Width(prefix)
	if available < 10 {
		available = 10
	}
	wrapLine := func(text string) []string {
		if strings.TrimSpace(text) == "" {
			return []string{text}
		}
		wrapped := wordwrap.String(text, available)
		if wrapped == "" {
			return []string{""}
		}
		return strings.Split(wrapped, "\n")
	}
	lines := make([]string, 0, len(msgLines))
	padding := strings.Repeat(" ", lipgloss.Width(prefix))
	paddingStyled := prefixStyle.Render(padding)
	firstLine := true
	lockedSuffix := ""
	if e.Immutable {
		lockedSuffix = " · locked"
	}
	for _, msgLine := range msgLines {
		segments := wrapLine(msgLine)
		for i, seg := range segments {
			content := seg
			if firstLine && i == 0 && lockedSuffix != "" {
				content = content + lockedSuffix
			}
			if firstLine && i == 0 {
				lines = append(lines, prefixStyle.Render(prefix)+messageStyle.Render(content))
				firstLine = false
				continue
			}
			lines = append(lines, paddingStyled+messageStyle.Render(content))
		}
	}
	if len(lines) == 0 {
		lines = append(lines, prefixStyle.Render(prefix))
	}
	return lines
}
