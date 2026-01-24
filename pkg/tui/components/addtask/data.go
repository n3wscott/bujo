package addtask

import (
	"strings"

	"tableflip.dev/bujo/pkg/collection"
)

func (m *Model) resolveTargetCollection() string {
	if len(m.collectionOptions) == 0 {
		return ""
	}
	if m.collectionIndex >= len(m.collectionOptions) {
		m.collectionIndex = len(m.collectionOptions) - 1
	}
	if m.collectionIndex < 0 {
		m.collectionIndex = 0
	}
	meta := m.collectionOptions[m.collectionIndex].Meta
	return meta.Name
}

func (m *Model) refreshCollections() {
	metas := m.cache.CollectionsMeta()
	currentID := ""
	if len(m.collectionOptions) > 0 && m.collectionIndex >= 0 && m.collectionIndex < len(m.collectionOptions) {
		currentID = m.collectionOptions[m.collectionIndex].ID
	}
	opts := make([]collectionOption, 0, len(metas)+1)
	placeholderID := strings.TrimSpace(m.initialCollectionID)
	placeholderLabel := strings.TrimSpace(m.initialCollectionLabel)
	if placeholderLabel == "" && placeholderID != "" {
		placeholderLabel = collectionLabel(placeholderID)
	}

	for _, meta := range metas {
		option := collectionOption{
			ID:    meta.Name,
			Label: collectionLabel(meta.Name),
			Meta:  meta,
		}
		if strings.EqualFold(option.ID, placeholderID) {
			// Prefer the real metadata label when it exists.
			if placeholderLabel != "" {
				option.Label = placeholderLabel
			}
		}
		opts = append(opts, option)
	}

	if placeholderID != "" {
		found := false
		for _, opt := range opts {
			if strings.EqualFold(opt.ID, placeholderID) {
				found = true
				break
			}
		}
		if !found {
			opts = append(opts, collectionOption{
				ID:    placeholderID,
				Label: placeholderLabel,
				Meta: collection.Meta{
					Name: placeholderID,
				},
			})
			if currentID == "" {
				currentID = placeholderID
			}
		}
	}

	m.collectionOptions = opts
	if len(m.collectionOptions) == 0 {
		m.collectionIndex = 0
		m.updatePrompt()
		return
	}
	selected := false
	if currentID != "" {
		selected = m.selectCollectionByID(currentID)
	}
	if !selected && placeholderID != "" {
		selected = m.selectCollectionByID(placeholderID)
	}
	if !selected {
		m.collectionIndex = clampIndex(m.collectionIndex, len(m.collectionOptions))
	} else if m.collectionIndex >= len(m.collectionOptions) {
		m.collectionIndex = len(m.collectionOptions) - 1
	}
	m.updatePrompt()
}

func (m *Model) refreshParentOptions() {
	target := m.resolveTargetCollection()
	if target == "" {
		m.parentOptions = []parentOption{{ID: "", Label: "(none)"}}
		m.parentIndex = 0
		m.updatePrompt()
		return
	}
	section, ok := m.cache.SectionSnapshot(target)
	if !ok {
		m.parentOptions = []parentOption{{ID: "", Label: "(none)"}}
		m.parentIndex = 0
		m.updatePrompt()
		return
	}
	opts := []parentOption{{ID: "", Label: "(none)"}}
	for _, bullet := range section.Bullets {
		label := bullet.Label
		if strings.TrimSpace(label) == "" {
			label = bullet.ID
		}
		opts = append(opts, parentOption{
			ID:    bullet.ID,
			Label: label,
		})
	}
	m.parentOptions = opts
	m.parentIndex = clampIndex(m.parentIndex, len(m.parentOptions))
	m.updatePrompt()
}

func (m *Model) selectCollectionByID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	for idx, opt := range m.collectionOptions {
		if strings.EqualFold(opt.ID, id) {
			m.collectionIndex = idx
			return true
		}
	}
	return false
}

func (m *Model) selectParentByID(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		m.parentIndex = 0
		return
	}
	for idx, opt := range m.parentOptions {
		if strings.EqualFold(opt.ID, id) {
			m.parentIndex = idx
			return
		}
	}
}
