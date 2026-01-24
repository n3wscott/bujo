package detail

// ensureScrollVisible adjusts scroll offset so active row is visible in viewport.
func (s *State) ensureScrollVisible() {
	height := s.viewHeight
	if height <= 0 {
		return
	}
	if len(s.sections) == 0 || s.sectionIndex < 0 || s.sectionIndex >= len(s.sections) {
		return
	}
	contentTop := 0
	for i := 0; i < s.sectionIndex; i++ {
		contentTop += s.sectionHeight(i)
	}
	section := s.sections[s.sectionIndex]
	var entryTop, entryBottom int
	if len(section.Entries) == 0 {
		entryTop = contentTop + 1
		entryBottom = entryTop
	} else {
		if s.sectionIndex >= len(s.entryOffsets) || s.entryOffsets[s.sectionIndex] == nil {
			s.sectionHeight(s.sectionIndex)
		}
		lineOffset := 1
		entryHeight := 1
		if s.sectionIndex < len(s.entryOffsets) {
			offsets := s.entryOffsets[s.sectionIndex]
			if s.entryIndex >= 0 && s.entryIndex < len(offsets) {
				if offsets[s.entryIndex] > 0 {
					lineOffset = offsets[s.entryIndex]
				}
			}
		}
		if s.sectionIndex < len(s.entryHeights) {
			heights := s.entryHeights[s.sectionIndex]
			if s.entryIndex >= 0 && s.entryIndex < len(heights) {
				if heights[s.entryIndex] > 0 {
					entryHeight = heights[s.entryIndex]
				}
			}
		}
		entryTop = contentTop + lineOffset
		entryBottom = entryTop + entryHeight - 1
	}
	if entryTop < s.scrollOffset {
		s.scrollOffset = entryTop
	}
	viewBottom := s.scrollOffset + height - 1
	if entryBottom > viewBottom {
		s.scrollOffset = entryBottom - height + 1
		if s.scrollOffset < 0 {
			s.scrollOffset = 0
		}
	}
	sectionTop := contentTop
	if s.scrollOffset > sectionTop {
		if entryTop-sectionTop <= 1 {
			s.scrollOffset = sectionTop
		}
	}
}

func (s *State) sectionHeight(idx int) int {
	if idx < 0 || idx >= len(s.sections) {
		return 0
	}
	if s.cachedHeights[idx] >= 0 {
		return s.cachedHeights[idx]
	}
	lines := s.renderSection(idx)
	return len(lines)
}

func (s *State) maxScrollOffset(height int) int {
	if height <= 0 {
		return 0
	}
	total := 0
	for i := range s.sections {
		total += s.sectionHeight(i)
	}
	maxOffset := total - height
	if maxOffset < 0 {
		return 0
	}
	return maxOffset
}

func (s *State) viewHeightFor(height int) int {
	if height <= 0 {
		return 0
	}
	result := height
	if result > s.viewHeight {
		result = s.viewHeight
	}
	if result < 0 {
		result = 0
	}
	return result
}

func clampScrollOffset(offset, limit int) int {
	if offset < 0 {
		offset = 0
	}
	if limit < 0 {
		limit = 0
	}
	if offset > limit {
		return limit
	}
	return offset
}
