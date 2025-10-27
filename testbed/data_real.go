package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/store"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/uiutil"
)

var (
	realServiceOnce sync.Once
	realService     *app.Service
	realServiceErr  error
)

func loadRealService() (*app.Service, error) {
	realServiceOnce.Do(func() {
		persistence, err := store.Load(nil)
		if err != nil {
			realServiceErr = fmt.Errorf("load journal store: %w", err)
			return
		}
		realService = &app.Service{Persistence: persistence}
	})
	return realService, realServiceErr
}

func realCollectionsData() ([]*viewmodel.ParsedCollection, error) {
	svc, err := loadRealService()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	metas, err := svc.CollectionsMeta(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("load collection metadata: %w", err)
	}
	return viewmodel.BuildTree(metas), nil
}

func realDetailSections() ([]collectiondetail.Section, error) {
	svc, err := loadRealService()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	metas, err := svc.CollectionsMeta(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("load collection metadata: %w", err)
	}
	index := make(map[string]collection.Meta, len(metas))
	for _, meta := range metas {
		index[meta.Name] = meta
	}
	sort.SliceStable(metas, func(i, j int) bool {
		return compareCollectionMeta(metas[i], metas[j], index)
	})
	sections := make([]collectiondetail.Section, 0, len(metas))
	for _, meta := range metas {
		entries, err := svc.Entries(ctx, meta.Name)
		if err != nil {
			return nil, fmt.Errorf("load entries for %q: %w", meta.Name, err)
		}
		section := collectiondetail.Section{
			ID:    meta.Name,
			Title: sectionTitle(meta),
			// todo, we should use the sub collection title as the Subtitle?
			Bullets: buildDetailBullets(entries),
		}
		sections = append(sections, section)
	}
	return sections, nil
}

func sectionTitle(meta collection.Meta) string {
	name := strings.TrimSpace(meta.Name)
	if name == "" {
		return "Unnamed"
	}
	if strings.Contains(name, "/") {
		if formatted := uiutil.FormattedCollectionName(name); formatted != "" {
			return formatted
		}
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
	for _, e := range entries {
		if e == nil || e.ID == "" {
			continue
		}
		idMap[e.ID] = e
	}
	children := make(map[string][]*entry.Entry)
	hasParent := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e == nil || e.ID == "" {
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
		hasParent[e.ID] = true
	}
	roots := make([]*entry.Entry, 0, len(entries))
	for _, e := range entries {
		if e == nil || e.ID == "" {
			continue
		}
		if !hasParent[e.ID] {
			roots = append(roots, e)
		}
	}
	if len(roots) == 0 {
		roots = entries
	}
	bullets := make([]collectiondetail.Bullet, 0, len(roots))
	for _, root := range roots {
		if root == nil {
			continue
		}
		bullets = append(bullets, entryToDetailBullet(root, children, make(map[string]bool)))
	}
	return bullets
}

func entryToDetailBullet(e *entry.Entry, children map[string][]*entry.Entry, visited map[string]bool) collectiondetail.Bullet {
	if e == nil {
		return collectiondetail.Bullet{}
	}
	bullet := collectiondetail.Bullet{
		ID:        e.ID,
		Label:     uiutil.EntryLabel(e),
		Note:      e.Collection,
		Bullet:    e.Bullet,
		Signifier: e.Signifier,
		Created:   e.Created.Time,
	}
	if e.ID == "" || visited[e.ID] {
		return bullet
	}
	visited[e.ID] = true
	kids := children[e.ID]
	if len(kids) > 0 {
		bullet.Children = make([]collectiondetail.Bullet, 0, len(kids))
		for _, child := range kids {
			if child == nil {
				continue
			}
			bullet.Children = append(bullet.Children, entryToDetailBullet(child, children, visited))
		}
	}
	delete(visited, e.ID)
	return bullet
}

func compareCollectionMeta(a, b collection.Meta, index map[string]collection.Meta) bool {
	ap := parentCollectionName(a.Name)
	bp := parentCollectionName(b.Name)
	if ap != "" && ap == bp {
		if parent, ok := index[ap]; ok && parent.Type == collection.TypeDaily {
			ad := parseDayTimestamp(ap, a.Name)
			bd := parseDayTimestamp(bp, b.Name)
			if !ad.IsZero() || !bd.IsZero() {
				if ad.Equal(bd) {
					return strings.ToLower(a.Name) < strings.ToLower(b.Name)
				}
				if ad.IsZero() {
					return false
				}
				if bd.IsZero() {
					return true
				}
				return ad.Before(bd)
			}
		}
	}
	return strings.ToLower(a.Name) < strings.ToLower(b.Name)
}

func parentCollectionName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		return strings.TrimSpace(name[:idx])
	}
	return ""
}

func lastSegment(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		return strings.TrimSpace(name[idx+1:])
	}
	return name
}

func parseDayTimestamp(parent, child string) time.Time {
	parentSegment := lastSegment(parent)
	childSegment := lastSegment(child)
	if day := uiutil.ParseDay(parentSegment, childSegment); !day.IsZero() {
		return day
	}
	if day, err := time.Parse("January 2, 2006", childSegment); err == nil {
		return day
	}
	return time.Time{}
}
