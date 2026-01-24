package detail

// MoveEntry moves the cursor within entries, adjusting section when crossing boundaries.
func (s *State) MoveEntry(delta int) bool {
	if len(s.sections) == 0 {
		return false
	}
	if len(s.sections[s.sectionIndex].Entries) == 0 {
		return false
	}
	attempts := 0
	maxAttempts := len(s.sections) * 4
	for {
		section := &s.sections[s.sectionIndex]
		s.entryIndex += delta
		for s.entryIndex < 0 || s.entryIndex >= len(section.Entries) {
			if s.entryIndex < 0 {
				if s.sectionIndex == 0 {
					s.entryIndex = 0
					break
				}
				s.sectionIndex--
				section = &s.sections[s.sectionIndex]
				s.entryIndex = len(section.Entries) - 1
				if s.entryIndex < 0 {
					s.entryIndex = 0
					break
				}
			} else {
				if s.sectionIndex == len(s.sections)-1 {
					s.entryIndex = len(section.Entries) - 1
					if s.entryIndex < 0 {
						s.entryIndex = 0
					}
					break
				}
				s.sectionIndex++
				section = &s.sections[s.sectionIndex]
				s.entryIndex = 0
			}
		}
		if s.isVisibleEntry(s.sectionIndex, s.entryIndex) {
			break
		}
		deltaSign := 1
		if delta < 0 {
			deltaSign = -1
		}
		s.entryIndex += deltaSign
		attempts++
		if attempts > maxAttempts {
			return false
		}
	}
	s.ensureScrollVisible()
	return true
}

// MoveSection moves to another section and resets entry index.
func (s *State) MoveSection(delta int) bool {
	if len(s.sections) == 0 {
		return false
	}
	s.sectionIndex += delta
	if s.sectionIndex < 0 {
		s.sectionIndex = 0
	}
	if s.sectionIndex >= len(s.sections) {
		s.sectionIndex = len(s.sections) - 1
	}
	if len(s.sections[s.sectionIndex].Entries) == 0 {
		s.entryIndex = 0
	} else {
		if idx := s.firstVisibleIndex(s.sectionIndex); idx >= 0 {
			s.entryIndex = idx
		} else {
			s.entryIndex = s.clampedEntryIndex()
		}
	}
	s.ensureScrollVisible()
	return true
}

// SetCursor positions the cursor, clamping to available entries.
func (s *State) SetCursor(sectionIdx, entryIdx int) {
	if len(s.sections) == 0 {
		s.sectionIndex = 0
		s.entryIndex = 0
		return
	}
	if sectionIdx < 0 {
		sectionIdx = 0
	}
	if sectionIdx >= len(s.sections) {
		sectionIdx = len(s.sections) - 1
	}
	s.sectionIndex = sectionIdx
	s.entryIndex = entryIdx
	s.clampEntry()
	s.ensureVisibleCurrent()
	s.ensureScrollVisible()
}

// SetActive moves the cursor to the given collection and entry identifiers.
func (s *State) SetActive(collectionID, entryID string) {
	if len(s.sections) == 0 {
		s.sectionIndex = 0
		s.entryIndex = 0
		return
	}
	if collectionID != "" {
		if idx := s.indexOfCollection(collectionID); idx >= 0 {
			s.sectionIndex = idx
		}
	}
	s.clampEntry()
	if entryID != "" {
		section := s.sections[s.sectionIndex]
		for i, it := range section.Entries {
			if it.ID == entryID {
				s.entryIndex = i
				break
			}
		}
	}
	s.clampEntry()
	if len(s.sections[s.sectionIndex].Entries) > 0 {
		entry := s.sections[s.sectionIndex].Entries[s.entryIndex]
		if entry != nil {
			s.unfoldAncestors(s.sectionIndex, entry.ID)
		}
	}
	s.ensureVisibleCurrent()
	s.ensureScrollVisible()
}

func (s *State) indexOfCollection(collectionID string) int {
	for i, sec := range s.sections {
		if sec.CollectionID == collectionID {
			return i
		}
	}
	return -1
}

func (s *State) clampEntry() {
	if len(s.sections) == 0 {
		s.entryIndex = 0
		return
	}
	s.entryIndex = s.clampedEntryIndex()
}

func (s *State) invalidateHeights() {
	for i := range s.cachedHeights {
		s.cachedHeights[i] = -1
	}
	for i := range s.entryOffsets {
		s.entryOffsets[i] = nil
	}
	for i := range s.entryHeights {
		s.entryHeights[i] = nil
	}
}

func (s *State) clampedEntryIndex() int {
	if len(s.sections) == 0 {
		return 0
	}
	entries := len(s.sections[s.sectionIndex].Entries)
	if entries == 0 {
		return 0
	}
	idx := s.entryIndex
	if idx < 0 {
		idx = 0
	}
	if idx >= entries {
		idx = entries - 1
	}
	return idx
}

// ScrollToTop resets viewport to the first line.
func (s *State) ScrollToTop() {
	s.scrollOffset = 0
	s.sectionIndex = 0
	s.entryIndex = 0
}

// ClearSelection removes any active entry/section selection.
func (s *State) ClearSelection() {
	s.sectionIndex = -1
	s.entryIndex = -1
}

func (s *State) isVisibleEntry(sectionIdx, entryIdx int) bool {
	if sectionIdx < 0 || sectionIdx >= len(s.sections) {
		return false
	}
	section := s.sections[sectionIdx]
	if entryIdx < 0 || entryIdx >= len(section.Entries) {
		return false
	}
	item := section.Entries[entryIdx]
	if item == nil {
		return false
	}
	visited := make(map[string]bool)
	parentID := item.ParentID
	for parentID != "" {
		if visited[parentID] {
			break
		}
		visited[parentID] = true
		if s.folded[parentID] {
			return false
		}
		next, ok := s.parents[sectionIdx][parentID]
		if !ok {
			break
		}
		parentID = next
	}
	return true
}

func (s *State) depthOf(sectionIdx int, entryID string) int {
	depth := 0
	visited := make(map[string]bool)
	current := entryID
	for {
		parentID, ok := s.parents[sectionIdx][current]
		if !ok || parentID == "" {
			break
		}
		if visited[parentID] {
			break
		}
		visited[parentID] = true
		depth++
		current = parentID
	}
	return depth
}

func (s *State) hasChildren(sectionIdx int, entryID string) bool {
	if sectionIdx < 0 || sectionIdx >= len(s.children) {
		return false
	}
	return len(s.children[sectionIdx][entryID]) > 0
}

// EntryHasChildren reports whether the entry has visible children in the section.
func (s *State) EntryHasChildren(sectionIdx int, entryID string) bool {
	return s.hasChildren(sectionIdx, entryID)
}

// ToggleEntryFold records a fold state for an entry subtree.
func (s *State) ToggleEntryFold(entryID string, collapsed bool) {
	if s.folded == nil {
		s.folded = make(map[string]bool)
	}
	if collapsed {
		s.folded[entryID] = true
	} else {
		delete(s.folded, entryID)
	}
	s.invalidateHeights()
}

// EntryFolded reports whether an entry is currently collapsed.
func (s *State) EntryFolded(entryID string) bool {
	return s.folded[entryID]
}

func (s *State) ensureVisibleCurrent() {
	if len(s.sections) == 0 {
		s.sectionIndex = 0
		s.entryIndex = 0
		return
	}
	if len(s.sections[s.sectionIndex].Entries) == 0 {
		s.entryIndex = 0
		return
	}
	entry := s.sections[s.sectionIndex].Entries[s.entryIndex]
	if entry == nil {
		return
	}
	s.unfoldAncestors(s.sectionIndex, entry.ID)
}

func (s *State) firstVisibleIndex(sectionIdx int) int {
	if sectionIdx < 0 || sectionIdx >= len(s.sections) {
		return -1
	}
	section := s.sections[sectionIdx]
	for i, entry := range section.Entries {
		if entry == nil {
			continue
		}
		if s.isVisibleEntry(sectionIdx, i) {
			return i
		}
	}
	return -1
}

func (s *State) unfoldAncestors(sectionIdx int, entryID string) {
	if entryID == "" {
		return
	}
	visited := make(map[string]bool)
	parentID := entryID
	for parentID != "" {
		if visited[parentID] {
			break
		}
		visited[parentID] = true
		delete(s.folded, parentID)
		parentID = s.parents[sectionIdx][parentID]
	}
}

// RevealCollection scrolls to the collection by ID.
func (s *State) RevealCollection(collectionID string, preferFull bool, height int) {
	idx := s.indexOfCollection(collectionID)
	if idx == -1 {
		return
	}
	top := s.sectionTop(idx)
	viewport := s.viewHeightFor(height)
	sectionHeight := s.sectionHeight(idx)
	if preferFull && sectionHeight <= viewport {
		target := top + sectionHeight - viewport
		if target < 0 {
			target = 0
		}
		s.scrollOffset = target
		return
	}
	s.scrollOffset = clampScrollOffset(top, s.maxScrollOffset(viewport))
}
