package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tableflip.dev/bujo/pkg/collection"
	viewmodel "tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/tui/events"
)

func (m *Model) futureMoveCollections(ctx context.Context, now time.Time) ([]*viewmodel.ParsedCollection, error) {
	if m.service == nil {
		return nil, fmt.Errorf("service offline")
	}
	metas, err := m.service.CollectionsMeta(ctx, "")
	if err != nil {
		return nil, err
	}
	return futureCollectionsFromMetas(metas, now), nil
}

func futureCollectionsFromMetas(metas []collection.Meta, now time.Time) []*viewmodel.ParsedCollection {
	existing := make(map[string]collection.Meta, len(metas))
	for _, meta := range metas {
		name := strings.TrimSpace(meta.Name)
		if name == "" {
			continue
		}
		if meta.Type == "" {
			meta.Type = collection.TypeGeneric
		}
		existing[name] = meta
	}
	futureType := collection.TypeMonthly
	if meta, ok := existing["Future"]; ok && meta.Type != "" {
		futureType = meta.Type
	}
	treeMetas := []collection.Meta{{Name: "Future", Type: futureType}}
	base := startOfMonth(now)
	if base.IsZero() {
		base = startOfMonth(time.Now())
	}
	for i := 1; i <= 12; i++ {
		monthTime := base.AddDate(0, i, 0)
		monthName := monthTime.Format("January 2006")
		full := fmt.Sprintf("Future/%s", monthName)
		treeMetas = append(treeMetas, collection.Meta{Name: full, Type: collection.TypeGeneric})
	}
	roots := viewmodel.BuildTree(treeMetas)
	existingSet := make(map[string]collection.Meta, len(existing))
	for name, meta := range existing {
		existingSet[name] = meta
	}
	var futureNode *viewmodel.ParsedCollection
	stack := append([]*viewmodel.ParsedCollection(nil), roots...)
	for len(stack) > 0 {
		last := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if last == nil {
			continue
		}
		if last.ID == "Future" {
			futureNode = last
			last.Type = collection.TypeMonthly
			last.Exists = true
		} else if last.ParentID == "Future" {
			last.Type = collection.TypeGeneric
			if _, ok := existingSet[last.ID]; ok {
				last.Exists = true
			} else {
				last.Exists = false
			}
		} else {
			last.Exists = false
		}
		if len(last.Children) > 0 {
			stack = append(stack, last.Children...)
		}
	}
	if futureNode != nil {
		monthMap := make(map[string]*viewmodel.ParsedCollection, len(futureNode.Children))
		for _, child := range futureNode.Children {
			if child == nil {
				continue
			}
			monthMap[child.Name] = child
		}
		ordered := make([]*viewmodel.ParsedCollection, 0, len(monthMap))
		baseMonth := startOfMonth(now)
		if baseMonth.IsZero() {
			baseMonth = startOfMonth(time.Now())
		}
		for i := 1; i <= 12; i++ {
			slot := baseMonth.AddDate(0, i, 0)
			name := slot.Format("January 2006")
			if child, ok := monthMap[name]; ok {
				child.Priority = i
				child.SortKey = fmt.Sprintf("%02d-%s", i, strings.ToLower(name))
				ordered = append(ordered, child)
			}
		}
		futureNode.Children = ordered
	}
	return roots
}

func filterMoveCollections(collections []*viewmodel.ParsedCollection) []*viewmodel.ParsedCollection {
	if len(collections) == 0 {
		return nil
	}
	trimmed := make([]*viewmodel.ParsedCollection, 0, len(collections))
	for _, col := range collections {
		if col == nil {
			continue
		}
		if isFutureCollection(col.ID) {
			continue
		}
		clone := cloneParsedCollection(col)
		clone.Children = filterMoveCollections(clone.Children)
		trimmed = append(trimmed, clone)
	}
	return trimmed
}

func cloneParsedCollection(col *viewmodel.ParsedCollection) *viewmodel.ParsedCollection {
	if col == nil {
		return nil
	}
	clone := *col
	if len(col.Children) > 0 {
		clone.Children = make([]*viewmodel.ParsedCollection, len(col.Children))
		for i := range col.Children {
			clone.Children[i] = cloneParsedCollection(col.Children[i])
		}
	}
	return &clone
}

func isFutureCollection(id string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(id))
	if trimmed == "" {
		return false
	}
	if trimmed == "future" {
		return true
	}
	return strings.HasPrefix(trimmed, "future/")
}

func resolvedCollectionPath(ref events.CollectionRef) string {
	path := strings.TrimSpace(ref.ID)
	if path != "" {
		return path
	}
	name := strings.TrimSpace(ref.Name)
	parent := strings.TrimSpace(ref.ParentID)
	switch {
	case parent != "" && name != "":
		return strings.TrimSuffix(parent, "/") + "/" + name
	case name != "":
		return name
	default:
		return ""
	}
}
