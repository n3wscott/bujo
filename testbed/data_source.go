package main

import (
	"sort"
	"strings"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
)

func loadCollectionsData(useReal bool) ([]collection.Meta, []*viewmodel.ParsedCollection, error) {
	if useReal {
		return realCollectionsData()
	}
	metas, priorities := sampleCollectionData()
	parsed := viewmodel.BuildTree(metas, viewmodel.WithPriorities(priorities))
	return metas, parsed, nil
}

func loadDetailSectionsData(useReal bool, metas []collection.Meta, parsed []*viewmodel.ParsedCollection, hold int) ([]collectiondetail.Section, []heldBullet, error) {
	var (
		sections []collectiondetail.Section
		err      error
	)
	if useReal {
		sections, err = realDetailSections()
	} else {
		sections = sampleDetailSections()
	}
	if err != nil {
		return nil, nil, err
	}
	sections = sortSectionsLikeCollections(sections, parsed)
	metaIndex := make(map[string]collection.Meta, len(metas))
	for _, meta := range metas {
		metaIndex[strings.TrimSpace(meta.Name)] = meta
	}
	return applyHoldback(sections, hold, metaIndex)
}

func sortSectionsLikeCollections(sections []collectiondetail.Section, parsed []*viewmodel.ParsedCollection) []collectiondetail.Section {
	if len(sections) == 0 || len(parsed) == 0 {
		return sections
	}
	order := flattenCollectionOrder(parsed)
	if len(order) == 0 {
		return sections
	}
	index := make(map[string]int, len(order))
	for i, id := range order {
		key := strings.ToLower(strings.TrimSpace(id))
		if key == "" {
			continue
		}
		if _, exists := index[key]; !exists {
			index[key] = i
		}
	}
	sorted := append([]collectiondetail.Section(nil), sections...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(sorted[i].ID))
		right := strings.ToLower(strings.TrimSpace(sorted[j].ID))
		li, lok := index[left]
		ri, rok := index[right]
		switch {
		case lok && rok:
			if li == ri {
				return strings.ToLower(sorted[i].Title) < strings.ToLower(sorted[j].Title)
			}
			return li < ri
		case lok:
			return true
		case rok:
			return false
		default:
			return strings.ToLower(sorted[i].Title) < strings.ToLower(sorted[j].Title)
		}
	})
	return sorted
}

func flattenCollectionOrder(parsed []*viewmodel.ParsedCollection) []string {
	order := make([]string, 0, len(parsed))
	var walk func(nodes []*viewmodel.ParsedCollection)
	walk = func(nodes []*viewmodel.ParsedCollection) {
		for _, node := range nodes {
			if node == nil {
				continue
			}
			if node.ID != "" {
				order = append(order, node.ID)
			}
			if len(node.Children) > 0 {
				walk(node.Children)
			}
		}
	}
	walk(parsed)
	return order
}
