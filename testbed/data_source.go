package main

import (
	"sort"
	"strings"

	"tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
)

func loadCollectionsData(useReal bool) ([]*viewmodel.ParsedCollection, error) {
	if useReal {
		return realCollectionsData()
	}
	return sampleCollections(), nil
}

func loadDetailSectionsData(useReal bool, parsed []*viewmodel.ParsedCollection, hold int) ([]collectiondetail.Section, []heldBullet, error) {
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
	return applyHoldback(sections, hold)
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
