package collectiondetail

import (
	"sort"
	"strings"
)

func (m *Model) visibleSection() (int, bool) {
	if len(m.lines) == 0 || len(m.sections) == 0 {
		return -1, false
	}
	start := m.scroll
	if start < 0 {
		start = 0
	}
	if start >= len(m.lines) {
		start = len(m.lines) - 1
	}
	for i := start; i < len(m.lines); i++ {
		info := m.lines[i]
		if info.section < 0 || info.section >= len(m.sections) {
			continue
		}
		if info.kind == lineSpacer {
			continue
		}
		return info.section, true
	}
	return -1, false
}

func (m *Model) moveCursor(delta int) {
	if len(m.bulletLines) == 0 {
		m.cursor = -1
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.bulletLines) {
		m.cursor = len(m.bulletLines) - 1
	}
	m.ensureScroll()
	m.refreshActiveSection()
}

func (m *Model) ensureScroll() {
	if len(m.lines) == 0 {
		m.scroll = 0
		return
	}
	curLine := m.currentLineIndex()
	if curLine < 0 {
		if m.activeSection >= 0 && m.activeSection < len(m.sections) {
			for idx, info := range m.lines {
				if info.section == m.activeSection && info.kind == lineHeader {
					m.scrollToLine(idx)
					return
				}
			}
		}
		m.scroll = 0
		m.clampScroll()
		return
	}
	m.ensureLineVisible(curLine)
}

func (m *Model) pageSize() int {
	height := m.viewportContentHeight()
	if height <= 0 {
		return 10
	}
	if height <= 1 {
		return 1
	}
	return height - 1
}

func (m *Model) ensureLineVisible(target int) {
	if len(m.lines) == 0 {
		m.scroll = 0
		return
	}
	if target < 0 {
		target = 0
	}
	if target >= len(m.lines) {
		target = len(m.lines) - 1
	}
	contentHeight := m.viewportContentHeight()
	if contentHeight <= 0 {
		contentHeight = 1
	}
	topIdx := m.scroll
	if topIdx < 0 {
		topIdx = 0
	}
	if topIdx >= len(m.lines) {
		topIdx = len(m.lines) - 1
	}
	topOffset := m.lineOffset(topIdx)
	bottomOffset := topOffset
	remaining := contentHeight
	idx := topIdx
	for idx < len(m.lines) && remaining > 0 {
		h := m.lineHeight(idx)
		if h >= remaining {
			bottomOffset = m.lineOffset(idx) + remaining - 1
			remaining = 0
			break
		}
		remaining -= h
		bottomOffset = m.lineOffset(idx) + h - 1
		idx++
	}
	if remaining > 0 {
		bottomOffset = m.totalHeight - 1
	}
	lineTop := m.lineOffset(target)
	lineBottom := lineTop + m.lineHeight(target) - 1
	if lineTop < topOffset {
		m.scroll = target
		m.clampScroll()
		return
	}
	if lineBottom > bottomOffset {
		start := target
		total := m.lineHeight(target)
		if total <= 0 {
			total = 1
		}
		for start > 0 {
			prev := start - 1
			nextTotal := total + m.lineHeight(prev)
			if nextTotal > contentHeight {
				break
			}
			start = prev
			total = nextTotal
		}
		m.scroll = start
		m.clampScroll()
		return
	}
	m.clampScroll()
}

func (m *Model) viewportContentHeight() int {
	if m.height <= 0 {
		return 0
	}
	height := m.height - m.stickyHeaderHeight()
	if height <= 0 {
		return 1
	}
	return height
}

func (m *Model) stickyHeaderHeight() int {
	if m.height <= 0 {
		return 0
	}
	section, ok := m.visibleSection()
	if !ok {
		return 0
	}
	header := m.renderSectionHeader(section, m.sectionActive(section))
	lines := strings.Count(header, "\n") + 1
	if lines < 0 {
		return 0
	}
	if lines >= m.height {
		return m.height - 1
	}
	return lines
}

func (m *Model) rebuildLines() {
	m.lines = m.lines[:0]
	m.bulletLines = m.bulletLines[:0]
	for si, sec := range m.sections {
		m.lines = append(m.lines, lineInfo{section: si, kind: lineHeader})
		if len(sec.Bullets) == 0 {
			lineIdx := len(m.lines)
			m.lines = append(m.lines, lineInfo{section: si, kind: lineEmpty})
			m.bulletLines = append(m.bulletLines, lineIdx)
		} else {
			m.appendBulletLines(si, sec.Bullets, 0)
		}
		m.lines = append(m.lines, lineInfo{section: si, kind: lineSpacer})
	}
	if len(m.lines) > 0 {
		m.lines = m.lines[:len(m.lines)-1]
	}
	m.recomputeLineMetrics()
}

func (m *Model) appendBulletLines(section int, bullets []Bullet, depth int) {
	for bi := range bullets {
		lineIdx := len(m.lines)
		bullet := bullets[bi]
		info := lineInfo{section: section, kind: lineItem, indent: depth, bullet: bullet}
		m.lines = append(m.lines, info)
		m.bulletLines = append(m.bulletLines, lineIdx)
		if len(bullet.Children) > 0 {
			m.appendBulletLines(section, bullet.Children, depth+1)
		}
	}
}

func (m *Model) recomputeLineMetrics() {
	n := len(m.lines)
	if n == 0 {
		m.lineHeights = m.lineHeights[:0]
		m.lineOffsets = m.lineOffsets[:0]
		m.totalHeight = 0
		m.scroll = 0
		return
	}
	if cap(m.lineHeights) < n {
		m.lineHeights = make([]int, n)
	} else {
		m.lineHeights = m.lineHeights[:n]
	}
	if cap(m.lineOffsets) < n {
		m.lineOffsets = make([]int, n)
	} else {
		m.lineOffsets = m.lineOffsets[:n]
	}
	offset := 0
	for i := 0; i < n; i++ {
		h := m.measureLineHeight(m.lines[i])
		if h <= 0 {
			h = 1
		}
		m.lineHeights[i] = h
		m.lineOffsets[i] = offset
		offset += h
	}
	m.totalHeight = offset
	m.clampScroll()
}

func (m *Model) measureLineHeight(info lineInfo) int {
	switch info.kind {
	case lineHeader:
		header := m.renderSectionHeader(info.section, false)
		return strings.Count(header, "\n") + 1
	case lineSpacer:
		return 1
	case lineEmpty:
		text := m.renderEmptyLine(info.section, false)
		return strings.Count(text, "\n") + 1
	case lineItem:
		prefix := m.composeBulletPrefix(info.indent, info.bullet, false)
		lines := m.renderBulletLines(prefix, info.bullet)
		if len(lines) == 0 {
			return 1
		}
		return len(lines)
	default:
		return 1
	}
}

func (m *Model) lineHeight(idx int) int {
	if idx < 0 || idx >= len(m.lineHeights) {
		return 0
	}
	h := m.lineHeights[idx]
	if h <= 0 {
		h = m.measureLineHeight(m.lines[idx])
		if h <= 0 {
			h = 1
		}
		m.lineHeights[idx] = h
	}
	return h
}

func (m *Model) lineOffset(idx int) int {
	if idx < 0 || idx >= len(m.lineOffsets) {
		return 0
	}
	return m.lineOffsets[idx]
}

func (m *Model) clampScroll() {
	if len(m.lines) == 0 {
		m.scroll = 0
		return
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
	if m.scroll >= len(m.lines) {
		m.scroll = len(m.lines) - 1
	}
	maxIdx := m.maxScrollIndex()
	if m.scroll > maxIdx {
		m.scroll = maxIdx
	}
}

func (m *Model) maxScrollIndex() int {
	if len(m.lines) == 0 {
		return 0
	}
	visible := m.viewportContentHeight()
	if visible <= 0 {
		return 0
	}
	if m.totalHeight <= visible {
		return 0
	}
	maxOffset := m.totalHeight - visible
	idx := sort.Search(len(m.lineOffsets), func(i int) bool {
		return m.lineOffsets[i] > maxOffset
	}) - 1
	if idx < 0 {
		idx = 0
	}
	return idx
}

func (m *Model) currentLineIndex() int {
	if m.cursor < 0 || m.cursor >= len(m.bulletLines) {
		return -1
	}
	return m.bulletLines[m.cursor]
}
