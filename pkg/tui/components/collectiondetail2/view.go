package collectiondetail2

import (
	"strings"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/muesli/reflow/wordwrap"

	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/tui/theme"
)

// View renders the component.
func (m *Model) View() string {
	if m.height <= 0 {
		m.height = 20
	}
	if m.width <= 0 {
		m.width = 80
	}

	lines := m.renderVisibleLines()
	return strings.Join(lines, "\n")
}

func (m *Model) renderLine(idx int, selected bool) string {
	if idx < 0 || idx >= len(m.lines) {
		return ""
	}
	info := m.lines[idx]
	if info.section < 0 || info.section >= len(m.sections) {
		return ""
	}
	switch info.kind {
	case lineHeader:
		return m.renderSectionHeader(info.section, m.sectionActive(info.section))
	case lineSpacer:
		return ""
	case lineEmpty:
		return m.renderEmptyLine(info.section, m.sectionActive(info.section))
	case lineItem:
		return m.renderBulletInfo(info, selected)
	default:
		return ""
	}
}

func (m *Model) renderSectionHeader(section int, highlight bool) string {
	sec := m.sections[section]
	style := lipgloss.NewStyle().Bold(true).Underline(true)
	if sec.Placeholder {
		style = style.Italic(true).Foreground(lipgloss.Color("244"))
	}
	if highlight {
		style = style.Inherit(theme.Default().Accent)
	}
	title := sec.Title
	if title == "" {
		title = "(untitled)"
	}
	if sec.Subtitle != "" {
		title = title + " ▸ " + sec.Subtitle
	}
	return style.Width(m.width).Render(title)
}

func (m *Model) renderEmptyLine(section int, highlight bool) string {
	if section < 0 || section >= len(m.sections) {
		return ""
	}
	sec := m.sections[section]
	message := "  <empty>"
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	if sec.Placeholder {
		message = "  (collection not yet created — add a bullet to save it)"
		style = style.Italic(true).Foreground(lipgloss.Color("244"))
	}
	if highlight {
		style = style.Inherit(theme.Default().Accent)
	}
	return style.Render(message)
}

func (m *Model) renderBulletInfo(info lineInfo, selected bool) string {
	item := info.bullet
	prefix := m.composeBulletPrefix(info.indent, item, selected && m.focused)
	lines := m.renderBulletLines(prefix, item)
	prefixStyle, messageStyle := m.bulletStyles(item)
	for i, line := range lines {
		if i == 0 {
			lines[i] = prefixStyle.Render(prefix) + messageStyle.Render(strings.TrimPrefix(line, prefix))
		} else {
			lines[i] = messageStyle.Render(line)
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) wrapBulletLines(prefix, text string) []string {
	prefixWidth := lipgloss.Width(prefix)
	if prefixWidth <= 0 {
		prefixWidth = 2
	}
	available := m.width - prefixWidth
	if available < 10 {
		available = 10
	}

	wrapLine := func(s string) []string {
		if strings.TrimSpace(s) == "" {
			return []string{""}
		}
		wrapped := wordwrap.String(s, available)
		if wrapped == "" {
			return []string{""}
		}
		return strings.Split(wrapped, "\n")
	}

	padding := strings.Repeat(" ", prefixWidth)
	lines := make([]string, 0, 4)
	firstLine := true
	for _, raw := range strings.Split(text, "\n") {
		segments := wrapLine(raw)
		for i, seg := range segments {
			if firstLine && i == 0 {
				lines = append(lines, prefix+seg)
				continue
			}
			lines = append(lines, padding+seg)
		}
		firstLine = false
	}
	if len(lines) == 0 {
		lines = append(lines, prefix)
	}
	return lines
}

func (m *Model) renderBulletLabel(item Bullet) string {
	label := stripBulletDecorations(item.Label, item)
	if strings.TrimSpace(label) == "" {
		label = "<empty>"
	}
	return label
}

func stripBulletDecorations(label string, item Bullet) string {
	trimmed := strings.TrimLeft(label, " \t")
	signifierGlyph := item.Signifier.Glyph()
	trimmed = stripLeadingToken(trimmed, item.Signifier.String())
	trimmed = stripLeadingToken(trimmed, signifierGlyph.Symbol)
	for _, alias := range signifierGlyph.Aliases {
		if len([]rune(strings.TrimSpace(alias))) == 1 {
			trimmed = stripLeadingToken(trimmed, alias)
		}
	}
	bulletGlyph := item.Bullet.Glyph()
	trimmed = stripLeadingToken(trimmed, bulletGlyph.Symbol)
	for _, alias := range bulletGlyph.Aliases {
		if len([]rune(strings.TrimSpace(alias))) == 1 {
			trimmed = stripLeadingToken(trimmed, alias)
		}
	}
	trimmed = stripLeadingToken(trimmed, item.Bullet.String())
	return strings.TrimLeft(trimmed, " \t")
}

func stripLeadingToken(label, token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return label
	}
	candidates := []string{
		token,
		token + " ",
		token + "\t",
	}
	for _, candidate := range candidates {
		if strings.HasPrefix(label, candidate) {
			remaining := strings.TrimPrefix(label, candidate)
			return strings.TrimLeft(remaining, " \t")
		}
	}
	return label
}

func (m *Model) renderBulletLines(prefix string, item Bullet) []string {
	text := m.renderBulletLabel(item)
	return m.wrapBulletLines(prefix, text)
}

func (m *Model) bulletStyles(item Bullet) (lipgloss.Style, lipgloss.Style) {
	prefixStyle := lipgloss.NewStyle()
	messageStyle := lipgloss.NewStyle()
	switch item.Bullet {
	case glyph.Completed, glyph.Irrelevant, glyph.MovedCollection, glyph.MovedFuture:
		prefixStyle = prefixStyle.Foreground(lipgloss.Color("241"))
		messageStyle = messageStyle.Foreground(lipgloss.Color("241"))
	}
	if item.Bullet == glyph.Irrelevant {
		messageStyle = messageStyle.Strikethrough(true)
	}
	return prefixStyle, messageStyle
}

func (m *Model) composeBulletPrefix(depth int, item Bullet, selected bool) string {
	caret := " "
	if selected {
		caret = theme.Default().Accent.Render("→")
	}
	signifier := item.Signifier.String()
	if signifier == "" {
		signifier = " "
	}
	indent := strings.Repeat("  ", depth)
	symbol := item.Bullet.Glyph().Symbol
	if symbol == "" {
		symbol = item.Bullet.String()
	}
	if symbol == "" {
		symbol = "-"
	}
	return caret + signifier + " " + indent + symbol + " "
}

func (m *Model) renderVisibleLines() []string {
	height := m.height
	if height <= 0 {
		height = 1
	}
	lines := make([]string, 0, height)

	stickySection, hasSticky := m.visibleSection()

	appendLines := func(text string) {
		if len(lines) >= height {
			return
		}
		if text == "" {
			lines = append(lines, "")
			return
		}
		for _, part := range strings.Split(text, "\n") {
			if len(lines) >= height {
				break
			}
			lines = append(lines, part)
		}
	}

	skippedHeader := hasSticky
	if hasSticky {
		header := m.renderSectionHeader(stickySection, m.sectionActive(stickySection))
		appendLines(header)
	}

	start := m.scroll
	activeLine := m.currentLineIndex()
	for i := start; i < len(m.lines) && len(lines) < height; i++ {
		info := m.lines[i]
		if hasSticky && skippedHeader && info.kind == lineHeader && info.section == stickySection {
			skippedHeader = false
			continue
		}
		appendLines(m.renderLine(i, i == activeLine))
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return lines
}
