package viewmodel

import (
	"time"

	"tableflip.dev/bujo/pkg/collection"
)

// FlattenOrder returns a pre-order traversal of the collection tree IDs.
func FlattenOrder(parsed []*ParsedCollection) []string {
	order := make([]string, 0, len(parsed))
	var walk func(list []*ParsedCollection)
	walk = func(list []*ParsedCollection) {
		for _, node := range list {
			if node == nil {
				continue
			}
			order = append(order, node.ID)
			if len(node.Children) > 0 {
				walk(node.Children)
			}
		}
	}
	walk(parsed)
	return order
}

// OrderedIDs returns collection IDs ordered by the canonical sort rules.
func OrderedIDs(metas []collection.Meta, now time.Time) []string {
	parsed := BuildTree(metas, WithNow(now))
	return FlattenOrder(parsed)
}
