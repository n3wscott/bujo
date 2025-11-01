package newapp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/uiutil"
)

type journalSnapshot struct {
	metas    []collection.Meta
	parsed   []*viewmodel.ParsedCollection
	sections []collectiondetail.Section
}

func buildJournalSnapshot(ctx context.Context, svc *app.Service) (journalSnapshot, error) {
	metas, err := svc.CollectionsMeta(ctx, "")
	if err != nil {
		return journalSnapshot{}, fmt.Errorf("load collection metadata: %w", err)
	}
	parsed := viewmodel.BuildTree(metas)
	sections := make([]collectiondetail.Section, 0, len(metas))
	for _, meta := range metas {
		entries, err := svc.Entries(ctx, meta.Name)
		if err != nil {
			return journalSnapshot{}, fmt.Errorf("load entries for %q: %w", meta.Name, err)
		}
		sections = append(sections, buildDetailSection(meta, entries))
	}
	sections = sortSectionsLikeCollections(sections, parsed)
	return journalSnapshot{metas: metas, parsed: parsed, sections: sections}, nil
}

func buildDetailSection(meta collection.Meta, entries []*entry.Entry) collectiondetail.Section {
	id := strings.TrimSpace(meta.Name)
	subtitle := string(meta.Type)
	if subtitle != "" {
		subtitle = strings.ToLower(subtitle)
	}
	section := collectiondetail.Section{
		ID:       id,
		Title:    sectionTitle(meta),
		Subtitle: subtitle,
		Bullets:  buildDetailBullets(entries),
	}
	if len(section.Bullets) == 0 {
		section.Placeholder = true
	}
	return section
}

func sectionTitle(meta collection.Meta) string {
	name := strings.TrimSpace(meta.Name)
	if name == "" {
		return "Unnamed"
	}
	if formatted := uiutil.FormattedCollectionName(name); formatted != "" {
		return formatted
	}
	if friendly := uiutil.FriendlyCollectionName(name); friendly != "" {
		return friendly
	}
	return name
}

func buildDetailBullets(entries []*entry.Entry) []collectiondetail.Bullet {
	if len(entries) == 0 {
		return nil
	}
	idMap := make(map[string]*entry.Entry, len(entries))
	children := make(map[string][]*entry.Entry)
	hasParent := make(map[string]bool, len(entries))

	for _, e := range entries {
		if e == nil || strings.TrimSpace(e.ID) == "" {
			continue
		}
		id := strings.TrimSpace(e.ID)
		idMap[id] = e
	}
	for _, e := range entries {
		if e == nil || strings.TrimSpace(e.ID) == "" {
			continue
		}
		parentID := strings.TrimSpace(e.ParentID)
		if parentID == "" || parentID == e.ID {
			continue
		}
		if _, ok := idMap[parentID]; !ok {
			continue
		}
		children[parentID] = append(children[parentID], e)
		hasParent[strings.TrimSpace(e.ID)] = true
	}

	roots := make([]*entry.Entry, 0, len(entries))
	for _, e := range entries {
		if e == nil || strings.TrimSpace(e.ID) == "" {
			continue
		}
		if !hasParent[strings.TrimSpace(e.ID)] {
			roots = append(roots, e)
		}
	}
	if len(roots) == 0 {
		roots = entries
	}

	sort.SliceStable(roots, func(i, j int) bool {
		return roots[i].Created.Time.Before(roots[j].Created.Time)
	})

	bullets := make([]collectiondetail.Bullet, 0, len(roots))
	visited := make(map[string]bool, len(entries))
	for _, root := range roots {
		bullets = append(bullets, entryToBullet(root, children, visited))
	}
	return bullets
}

func entryToBullet(e *entry.Entry, children map[string][]*entry.Entry, visited map[string]bool) collectiondetail.Bullet {
	if e == nil {
		return collectiondetail.Bullet{}
	}
	id := strings.TrimSpace(e.ID)
	bullet := collectiondetail.Bullet{
		ID:        id,
		Label:     uiutil.EntryLabel(e),
		Note:      e.Collection,
		Bullet:    e.Bullet,
		Signifier: e.Signifier,
		Created:   e.Created.Time,
	}
	if id == "" || visited[id] {
		return bullet
	}
	visited[id] = true
	if kids := children[id]; len(kids) > 0 {
		sort.SliceStable(kids, func(i, j int) bool {
			return kids[i].Created.Time.Before(kids[j].Created.Time)
		})
		bullet.Children = make([]collectiondetail.Bullet, 0, len(kids))
		for _, child := range kids {
			bullet.Children = append(bullet.Children, entryToBullet(child, children, visited))
		}
	}
	delete(visited, id)
	return bullet
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
