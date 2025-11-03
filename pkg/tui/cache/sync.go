package cache

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/events"
	"tableflip.dev/bujo/pkg/tui/uiutil"
)

// BuildSnapshot loads collection metadata and entry sections from the supplied
// service, assembling a cache snapshot that mirrors on-disk state.
func BuildSnapshot(ctx context.Context, svc *app.Service) (Snapshot, error) {
	if svc == nil {
		return Snapshot{}, errors.New("cache: service unavailable")
	}
	metas, err := svc.CollectionsMeta(ctx, "")
	if err != nil {
		return Snapshot{}, fmt.Errorf("load collection metadata: %w", err)
	}
	parsed := viewmodel.BuildTree(metas)
	sections := make([]collectiondetail.Section, 0, len(metas))
	for _, meta := range metas {
		collectionID := strings.TrimSpace(meta.Name)
		entries, err := svc.Entries(ctx, collectionID)
		if err != nil {
			return Snapshot{}, fmt.Errorf("load entries for %q: %w", collectionID, err)
		}
		sections = append(sections, buildSection(meta, entries))
	}
	sections = sortSectionsLikeCollections(sections, parsed)
	return Snapshot{
		Metas:       metas,
		Collections: parsed,
		Sections:    sections,
	}, nil
}

// ApplySnapshot reconciles the cache with the provided snapshot, emitting
// collection/bullet change events for any detected differences.
func (c *Cache) ApplySnapshot(snapshot Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.applySnapshotLocked(snapshot)
}

// SyncCollection refreshes a single collection detail section from the
// configured service, emitting bullet diff events.
func (c *Cache) SyncCollection(ctx context.Context, collectionID string) error {
	svc := c.currentService()
	if svc == nil {
		return errors.New("cache: service unavailable")
	}
	trimmed := strings.TrimSpace(collectionID)
	if trimmed == "" {
		return errors.New("cache: collection id required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	entries, err := svc.Entries(ctx, trimmed)
	if err != nil {
		return fmt.Errorf("cache: load entries for %q: %w", trimmed, err)
	}

	meta, ok := c.metaForCollection(trimmed)
	if !ok {
		metas, err := svc.CollectionsMeta(ctx, trimmed)
		if err != nil {
			return fmt.Errorf("cache: load metadata for %q: %w", trimmed, err)
		}
		for _, candidate := range metas {
			if strings.EqualFold(candidate.Name, trimmed) {
				meta = candidate
				ok = true
				break
			}
		}
		if !ok {
			meta = collection.Meta{Name: trimmed, Type: collection.TypeGeneric}
		}
	}

	section := buildSection(meta, entries)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.applySectionLocked(section)
	return nil
}

func (c *Cache) applySnapshotLocked(snapshot Snapshot) {
	normalizedMetas := normalizeMetas(snapshot.Metas)
	newCollections := viewmodel.BuildTree(normalizedMetas)
	newSections := cloneSections(snapshot.Sections)

	oldMetas := cloneMetas(c.metas)
	oldSections := cloneSections(c.sections)

	c.diffMetasLocked(oldMetas, normalizedMetas)
	c.metas = normalizedMetas
	c.collections = newCollections

	c.diffSectionsLocked(oldSections, snapshot.Sections)
	c.sections = newSections

	c.templates = make(map[string]sectionTemplate, len(newSections))
	if c.entries == nil {
		c.entries = make(map[string][]collectiondetail.Bullet, len(newSections))
	} else {
		for k := range c.entries {
			delete(c.entries, k)
		}
	}
	for _, sec := range newSections {
		c.registerTemplate(sec)
		c.entries[sec.ID] = cloneBullets(sec.Bullets)
	}

	c.emitOrderLocked()
}

func (c *Cache) applySectionLocked(section collectiondetail.Section) {
	section.Bullets = dedupeBullets(section.Bullets)
	idx := c.sectionIndex(section.ID)
	if idx < 0 {
		c.sections = append(c.sections, cloneSection(section))
		c.registerTemplate(section)
		c.diffSectionBulletsLocked(collectiondetail.Section{ID: section.ID, Title: section.Title, Subtitle: section.Subtitle}, section)
	} else {
		oldSection := c.sections[idx]
		c.diffSectionBulletsLocked(oldSection, section)
		c.sections[idx] = cloneSection(section)
		c.registerTemplate(section)
	}
	if c.entries == nil {
		c.entries = make(map[string][]collectiondetail.Bullet)
	}
	c.entries[section.ID] = cloneBullets(section.Bullets)
}

func (c *Cache) diffMetasLocked(oldMetas, newMetas []collection.Meta) {
	oldMap := make(map[string]collection.Meta, len(oldMetas))
	for _, meta := range oldMetas {
		key := strings.ToLower(strings.TrimSpace(meta.Name))
		if key != "" {
			oldMap[key] = meta
		}
	}
	newMap := make(map[string]collection.Meta, len(newMetas))
	for _, meta := range newMetas {
		key := strings.ToLower(strings.TrimSpace(meta.Name))
		if key != "" {
			newMap[key] = meta
		}
	}

	for key, meta := range newMap {
		if previous, exists := oldMap[key]; !exists {
			c.emit(events.CollectionChangeMsg{
				Component: c.component,
				Action:    events.ChangeCreate,
				Current:   collectionRef(meta),
			})
		} else if previous.Type != meta.Type {
			prevRef := collectionRef(previous)
			currRef := collectionRef(meta)
			c.emit(events.CollectionChangeMsg{
				Component: c.component,
				Action:    events.ChangeUpdate,
				Current:   currRef,
				Previous:  &prevRef,
			})
		}
	}

	for key, meta := range oldMap {
		if _, exists := newMap[key]; !exists {
			c.emit(events.CollectionChangeMsg{
				Component: c.component,
				Action:    events.ChangeDelete,
				Current:   collectionRef(meta),
			})
		}
	}
}

func (c *Cache) diffSectionsLocked(oldSections, newSections []collectiondetail.Section) {
	oldMap := make(map[string]collectiondetail.Section, len(oldSections))
	for _, sec := range oldSections {
		key := strings.ToLower(strings.TrimSpace(sec.ID))
		if key != "" {
			oldMap[key] = sec
		}
	}
	newMap := make(map[string]collectiondetail.Section, len(newSections))
	for _, sec := range newSections {
		key := strings.ToLower(strings.TrimSpace(sec.ID))
		if key != "" {
			newMap[key] = sec
		}
	}

	for key, sec := range newMap {
		if old, exists := oldMap[key]; exists {
			c.diffSectionBulletsLocked(old, sec)
		} else {
			c.diffSectionBulletsLocked(collectiondetail.Section{ID: sec.ID, Title: sec.Title, Subtitle: sec.Subtitle}, sec)
		}
	}

	for key, sec := range oldMap {
		if _, exists := newMap[key]; !exists {
			c.emitSectionDeletion(sec)
		}
	}
}

func (c *Cache) diffSectionBulletsLocked(oldSec, newSec collectiondetail.Section) {
	newSet := make(map[string]bulletState)
	collectBulletStates(newSec.Bullets, "", newSet)

	oldSet := make(map[string]bulletState)
	collectBulletStates(oldSec.Bullets, "", oldSet)

	var walkNew func([]collectiondetail.Bullet, string)
	walkNew = func(list []collectiondetail.Bullet, parent string) {
		for _, bullet := range list {
			id := strings.TrimSpace(bullet.ID)
			if id != "" {
				state := bulletState{bullet: bullet, parent: parent}
				if previous, exists := oldSet[id]; !exists {
					c.emitBulletChange(events.ChangeCreate, newSec, state)
				} else if bulletChanged(previous, state) {
					c.emitBulletChange(events.ChangeUpdate, newSec, state)
				}
			}
			if len(bullet.Children) > 0 {
				walkNew(bullet.Children, bullet.ID)
			}
		}
	}
	walkNew(newSec.Bullets, "")

	var walkOld func([]collectiondetail.Bullet, string)
	walkOld = func(list []collectiondetail.Bullet, parent string) {
		for _, bullet := range list {
			id := strings.TrimSpace(bullet.ID)
			if id != "" {
				if _, exists := newSet[id]; !exists {
					c.emitBulletChange(events.ChangeDelete, oldSec, bulletState{bullet: bullet, parent: parent})
				}
			}
			if len(bullet.Children) > 0 {
				walkOld(bullet.Children, bullet.ID)
			}
		}
	}
	walkOld(oldSec.Bullets, "")
}

func (c *Cache) emitSectionDeletion(sec collectiondetail.Section) {
	var walk func([]collectiondetail.Bullet)
	walk = func(list []collectiondetail.Bullet) {
		for _, bullet := range list {
			if strings.TrimSpace(bullet.ID) != "" {
				c.emitBulletChange(events.ChangeDelete, sec, bulletState{bullet: bullet})
			}
			if len(bullet.Children) > 0 {
				walk(bullet.Children)
			}
		}
	}
	walk(sec.Bullets)
}

func (c *Cache) emitBulletChange(action events.ChangeType, sec collectiondetail.Section, state bulletState) {
	ref := events.CollectionViewRef{
		ID:       sec.ID,
		Title:    sec.Title,
		Subtitle: sec.Subtitle,
	}
	bulletRef := events.BulletRef{
		ID:        state.bullet.ID,
		Label:     state.bullet.Label,
		Note:      state.bullet.Note,
		Bullet:    state.bullet.Bullet,
		Signifier: state.bullet.Signifier,
	}
	var meta map[string]string
	if action == events.ChangeCreate && strings.TrimSpace(state.parent) != "" {
		meta = map[string]string{parentMetaKey: state.parent}
	}
	c.emit(events.BulletChangeMsg{
		Component:  c.component,
		Action:     action,
		Collection: ref,
		Bullet:     bulletRef,
		Meta:       meta,
	})
}

func (c *Cache) createBulletPersisted(ctx context.Context, svc *app.Service, collectionID string, bullet collectiondetail.Bullet, meta map[string]string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	target := strings.TrimSpace(collectionID)
	if target == "" {
		return errors.New("cache: collection id required")
	}
	label := strings.TrimSpace(bullet.Label)
	if label == "" {
		return errors.New("cache: bullet label required")
	}

	entry, err := svc.Add(ctx, target, bullet.Bullet, label, bullet.Signifier)
	if err != nil {
		return fmt.Errorf("cache: add entry: %w", err)
	}
	parentID := ""
	if meta != nil {
		parentID = strings.TrimSpace(meta[parentMetaKey])
	}
	if parentID != "" && strings.TrimSpace(entry.ParentID) != parentID {
		if _, err := svc.SetParent(ctx, entry.ID, parentID); err != nil {
			return fmt.Errorf("cache: set parent: %w", err)
		}
	}
	return c.SyncCollection(ctx, target)
}

func (c *Cache) metaForCollection(name string) (collection.Meta, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, meta := range c.metas {
		if strings.EqualFold(meta.Name, name) {
			return meta, true
		}
	}
	return collection.Meta{}, false
}

func buildSection(meta collection.Meta, entries []*entry.Entry) collectiondetail.Section {
	id := strings.TrimSpace(meta.Name)
	subtitle := string(meta.Type)
	if subtitle != "" {
		subtitle = strings.ToLower(subtitle)
	}
	bullets := buildBullets(entries)
	bullets = dedupeBullets(bullets)
	section := collectiondetail.Section{
		ID:       id,
		Title:    sectionTitle(meta),
		Subtitle: subtitle,
		Bullets:  bullets,
	}
	if len(section.Bullets) == 0 {
		section.Placeholder = true
	}
	return section
}

func buildBullets(entries []*entry.Entry) []collectiondetail.Bullet {
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

func dedupeBullets(list []collectiondetail.Bullet) []collectiondetail.Bullet {
	if len(list) == 0 {
		return nil
	}
	result := make([]collectiondetail.Bullet, 0, len(list))
	index := make(map[string]int, len(list))
	for _, bullet := range list {
		bullet.Children = dedupeBullets(bullet.Children)
		id := strings.TrimSpace(bullet.ID)
		if id == "" {
			result = append(result, bullet)
			continue
		}
		if idx, exists := index[id]; exists {
			result[idx] = mergeDetailBullet(result[idx], bullet)
			continue
		}
		index[id] = len(result)
		result = append(result, bullet)
	}
	return result
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
	var walk func(list []*viewmodel.ParsedCollection)
	walk = func(list []*viewmodel.ParsedCollection) {
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

type bulletState struct {
	bullet collectiondetail.Bullet
	parent string
}

func collectBulletStates(list []collectiondetail.Bullet, parent string, out map[string]bulletState) {
	for _, bullet := range list {
		id := strings.TrimSpace(bullet.ID)
		if id != "" {
			out[id] = bulletState{bullet: bullet, parent: parent}
		}
		if len(bullet.Children) > 0 {
			collectBulletStates(bullet.Children, bullet.ID, out)
		}
	}
}

func bulletChanged(oldState, newState bulletState) bool {
	if strings.TrimSpace(oldState.parent) != strings.TrimSpace(newState.parent) {
		return true
	}
	oldBullet := oldState.bullet
	newBullet := newState.bullet
	switch {
	case strings.TrimSpace(oldBullet.Label) != strings.TrimSpace(newBullet.Label):
		return true
	case strings.TrimSpace(oldBullet.Note) != strings.TrimSpace(newBullet.Note):
		return true
	case oldBullet.Bullet != newBullet.Bullet:
		return true
	case oldBullet.Signifier != newBullet.Signifier:
		return true
	default:
		return false
	}
}

func collectionRef(meta collection.Meta) events.CollectionRef {
	id := strings.TrimSpace(meta.Name)
	return events.CollectionRef{
		ID:       id,
		Name:     leafName(id),
		Type:     meta.Type,
		ParentID: parentPath(id),
	}
}

func cloneSection(section collectiondetail.Section) collectiondetail.Section {
	cloned := section
	cloned.Bullets = cloneBullets(section.Bullets)
	return cloned
}
