package cache

import (
	"sort"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/events"
)

// Snapshot exposes the current cached state.
type Snapshot struct {
	Collections []*viewmodel.ParsedCollection
	Sections    []collectiondetail.Section
}

// Cache maintains in-memory collections/entries and emits typed events on
// mutation. It mirrors the behavior of a Kubernetes-style informer cache:
// state lives locally, watchers subscribe to emitted events, and consumers read
// consistent snapshots without hitting the store.
type Cache struct {
	component events.ComponentID

	mu sync.RWMutex

	metas       []collection.Meta
	collections []*viewmodel.ParsedCollection

	sections []collectiondetail.Section
	entries  map[string][]collectiondetail.Bullet // keyed by collection ID

	templates map[string]sectionTemplate

	eventCh chan tea.Msg
}

type sectionTemplate struct {
	title    string
	subtitle string
}

// New creates an empty cache that will emit events using the provided
// ComponentID (falls back to "cache" if empty).
func New(component events.ComponentID) *Cache {
	if component == "" {
		component = events.ComponentID("cache")
	}
	return &Cache{
		component: component,
		eventCh:   make(chan tea.Msg, 64),
		entries:   make(map[string][]collectiondetail.Bullet),
	}
}

// Events exposes the cache event channel for Bubble Tea subscriptions.
func (c *Cache) Events() <-chan tea.Msg {
	return c.eventCh
}

// SetCollections seeds the cache with the provided metadata list. It rebuilds
// the parsed tree and emits no events (callers should emit ChangeMsgs if
// desired). Safe to call multiple times (it replaces state).
func (c *Cache) SetCollections(metas []collection.Meta) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metas = normalizeMetas(metas)
	c.collections = viewmodel.BuildTree(c.metas)
	c.emitOrderLocked()
}

// SetSections seeds the detail sections backing the collection detail pane.
func (c *Cache) SetSections(sections []collectiondetail.Section) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sections = cloneSections(sections)
	c.entries = map[string][]collectiondetail.Bullet{}
	for _, sec := range c.sections {
		c.entries[sec.ID] = cloneBullets(sec.Bullets)
		c.registerTemplate(sec)
	}
}

// Snapshot returns a copy of the current parsed collections and sections. The
// returned data should be treated as immutable by callers.
func (c *Cache) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	collections := cloneParsed(c.collections)
	sections := cloneSections(c.sections)
	return Snapshot{
		Collections: collections,
		Sections:    sections,
	}
}

// RegisterSectionTemplate stores presentation metadata for a collection so
// future dynamically created sections inherit the correct title/subtitle.
func (c *Cache) RegisterSectionTemplate(section collectiondetail.Section) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.registerTemplate(section)
}

// CreateCollection inserts metadata and emits a CollectionChangeMsg. It returns
// the parsed tree for the new state.
func (c *Cache) CreateCollection(meta collection.Meta) []*viewmodel.ParsedCollection {
	c.mu.Lock()
	defer c.mu.Unlock()
	meta = normalizeMeta(meta)
	if meta.Name == "" {
		return c.collections
	}
	if idx := findMetaIndex(c.metas, meta.Name); idx >= 0 {
		c.metas[idx] = meta
	} else {
		c.metas = append(c.metas, meta)
	}
	c.collections = viewmodel.BuildTree(c.metas)
	c.emit(events.CollectionChangeMsg{
		Component: c.component,
		Action:    events.ChangeCreate,
		Current:   events.CollectionRef{ID: meta.Name, Name: leafName(meta.Name), Type: meta.Type},
	})
	c.emitOrderLocked()
	return c.collections
}

// UpdateCollection applies metadata changes and emits an Update change message.
func (c *Cache) UpdateCollection(current collection.Meta, previous *collection.Meta) []*viewmodel.ParsedCollection {
	c.mu.Lock()
	defer c.mu.Unlock()
	curr := normalizeMeta(current)
	prevName := ""
	if previous != nil {
		prev := normalizeMeta(*previous)
		prevName = prev.Name
	}
	c.upsertMeta(curr, prevName)
	c.collections = viewmodel.BuildTree(c.metas)
	c.emit(events.CollectionChangeMsg{
		Component: c.component,
		Action:    events.ChangeUpdate,
		Current:   events.CollectionRef{ID: curr.Name, Name: leafName(curr.Name), Type: curr.Type},
		Previous: &events.CollectionRef{
			ID:   prevName,
			Name: leafName(prevName),
		},
	})
	c.emitOrderLocked()
	return c.collections
}

// DeleteCollection removes a collection (and children) and emits a delete
// change message.
func (c *Cache) DeleteCollection(name string) []*viewmodel.ParsedCollection {
	c.mu.Lock()
	defer c.mu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" {
		return c.collections
	}
	c.removeMeta(name)
	c.collections = viewmodel.BuildTree(c.metas)
	c.emit(events.CollectionChangeMsg{
		Component: c.component,
		Action:    events.ChangeDelete,
		Current:   events.CollectionRef{ID: name, Name: leafName(name)},
	})
	c.emitOrderLocked()
	return c.collections
}

// CreateBullet appends a bullet to the specified collection, emits a bullet
// change message, and returns the updated section.
func (c *Cache) CreateBullet(collectionID string, bullet collectiondetail.Bullet) {
	c.mutateBullet(collectionID, bullet, events.ChangeCreate, nil)
}

// UpdateBullet replaces an existing bullet (matched by ID) and emits an update.
func (c *Cache) UpdateBullet(collectionID string, bullet collectiondetail.Bullet) {
	c.mutateBullet(collectionID, bullet, events.ChangeUpdate, nil)
}

// DeleteBullet removes the specified bullet ID from the collection and emits a
// delete change message.
func (c *Cache) DeleteBullet(collectionID, bulletID string) {
	if bulletID == "" {
		return
	}
	c.mutateBullet(collectionID, collectiondetail.Bullet{ID: bulletID}, events.ChangeDelete, nil)
}

func (c *Cache) mutateBullet(collectionID string, bullet collectiondetail.Bullet, action events.ChangeType, meta map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	collectionID = strings.TrimSpace(collectionID)
	if collectionID == "" {
		return
	}
	sectionIdx := c.sectionIndex(collectionID)
	if sectionIdx < 0 {
		sectionIdx = c.createSectionIfMissing(collectionID)
	}
	if sectionIdx < 0 {
		return
	}
	switch action {
	case events.ChangeCreate:
		c.sections[sectionIdx].Bullets = append(c.sections[sectionIdx].Bullets, bullet)
	case events.ChangeUpdate:
		updateBulletByID(&c.sections[sectionIdx].Bullets, bullet)
	case events.ChangeDelete:
		removeBulletByID(&c.sections[sectionIdx].Bullets, bullet.ID)
	}
	ref := events.CollectionViewRef{
		ID:       c.sections[sectionIdx].ID,
		Title:    c.sections[sectionIdx].Title,
		Subtitle: c.sections[sectionIdx].Subtitle,
	}
	c.emit(events.BulletChangeMsg{
		Component:  c.component,
		Action:     action,
		Collection: ref,
		Bullet: events.BulletRef{
			ID:        bullet.ID,
			Label:     bullet.Label,
			Note:      bullet.Note,
			Bullet:    bullet.Bullet,
			Signifier: bullet.Signifier,
		},
		Meta: meta,
	})
}

func (c *Cache) sectionIndex(id string) int {
	for idx, sec := range c.sections {
		if strings.EqualFold(sec.ID, id) {
			return idx
		}
	}
	return -1
}

func (c *Cache) createSectionIfMissing(id string) int {
	template := c.templates[id]
	title := template.title
	if title == "" {
		title = leafName(id)
	}
	sec := collectiondetail.Section{
		ID:       id,
		Title:    title,
		Subtitle: template.subtitle,
	}
	c.sections = append(c.sections, sec)
	return len(c.sections) - 1
}

func (c *Cache) upsertMeta(meta collection.Meta, prev string) {
	if prev != "" {
		if idx := findMetaIndex(c.metas, prev); idx >= 0 {
			c.metas[idx] = meta
			return
		}
	}
	if idx := findMetaIndex(c.metas, meta.Name); idx >= 0 {
		c.metas[idx] = meta
		return
	}
	c.metas = append(c.metas, meta)
}

func (c *Cache) removeMeta(name string) {
	if len(c.metas) == 0 {
		return
	}
	filtered := c.metas[:0]
	for _, meta := range c.metas {
		if meta.Name == name || strings.HasPrefix(meta.Name, name+"/") {
			continue
		}
		filtered = append(filtered, meta)
	}
	c.metas = filtered
}

func (c *Cache) registerTemplate(section collectiondetail.Section) {
	id := strings.TrimSpace(section.ID)
	if id == "" {
		return
	}
	if c.templates == nil {
		c.templates = make(map[string]sectionTemplate)
	}
	title := strings.TrimSpace(section.Title)
	if title == "" {
		title = leafName(id)
	}
	c.templates[id] = sectionTemplate{
		title:    title,
		subtitle: strings.TrimSpace(section.Subtitle),
	}
}

func (c *Cache) emitOrderLocked() {
	order := flattenIDs(c.collections)
	if len(order) == 0 {
		return
	}
	c.emit(events.CollectionOrderMsg{
		Component: c.component,
		Order:     append([]string(nil), order...),
	})
}

func flattenIDs(nodes []*viewmodel.ParsedCollection) []string {
	if len(nodes) == 0 {
		return nil
	}
	order := make([]string, 0, len(nodes))
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
	walk(nodes)
	return order
}

func (c *Cache) emit(msg tea.Msg) {
	select {
	case c.eventCh <- msg:
	default:
	}
}

func normalizeMetas(metas []collection.Meta) []collection.Meta {
	if len(metas) == 0 {
		return nil
	}
	out := make([]collection.Meta, 0, len(metas))
	seen := make(map[string]struct{}, len(metas))
	for _, meta := range metas {
		n := normalizeMeta(meta)
		if n.Name == "" {
			continue
		}
		if _, exists := seen[n.Name]; exists {
			continue
		}
		seen[n.Name] = struct{}{}
		out = append(out, n)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func normalizeMeta(meta collection.Meta) collection.Meta {
	meta.Name = strings.TrimSpace(meta.Name)
	if meta.Type == "" {
		meta.Type = collection.TypeGeneric
	}
	return meta
}

func findMetaIndex(list []collection.Meta, name string) int {
	for idx, meta := range list {
		if strings.EqualFold(meta.Name, name) {
			return idx
		}
	}
	return -1
}

func cloneParsed(nodes []*viewmodel.ParsedCollection) []*viewmodel.ParsedCollection {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]*viewmodel.ParsedCollection, len(nodes))
	for i, node := range nodes {
		if node == nil {
			continue
		}
		cloned := *node
		cloned.Children = cloneParsed(node.Children)
		out[i] = &cloned
	}
	return out
}

func cloneSections(sections []collectiondetail.Section) []collectiondetail.Section {
	if len(sections) == 0 {
		return nil
	}
	out := make([]collectiondetail.Section, len(sections))
	for i := range sections {
		out[i] = sections[i]
		out[i].Bullets = cloneBullets(sections[i].Bullets)
	}
	return out
}

func cloneBullets(list []collectiondetail.Bullet) []collectiondetail.Bullet {
	if len(list) == 0 {
		return nil
	}
	out := make([]collectiondetail.Bullet, len(list))
	for i := range list {
		out[i] = list[i]
		out[i].Children = cloneBullets(list[i].Children)
	}
	return out
}

func leafName(name string) string {
	if name == "" {
		return ""
	}
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

func updateBulletByID(list *[]collectiondetail.Bullet, updated collectiondetail.Bullet) bool {
	if list == nil || updated.ID == "" {
		return false
	}
	items := *list
	for i := range items {
		if items[i].ID == updated.ID {
			items[i] = mergeDetailBullet(items[i], updated)
			return true
		}
		if len(items[i].Children) > 0 {
			if updateBulletByID(&items[i].Children, updated) {
				return true
			}
		}
	}
	return false
}

func mergeDetailBullet(existing, updated collectiondetail.Bullet) collectiondetail.Bullet {
	if updated.Label != "" {
		existing.Label = updated.Label
	}
	if updated.Note != "" {
		existing.Note = updated.Note
	}
	if updated.Bullet != "" {
		existing.Bullet = updated.Bullet
	}
	if updated.Signifier != "" {
		existing.Signifier = updated.Signifier
	}
	if !updated.Created.IsZero() {
		existing.Created = updated.Created
	}
	if len(updated.Children) > 0 {
		existing.Children = cloneBullets(updated.Children)
	}
	return existing
}

func removeBulletByID(list *[]collectiondetail.Bullet, id string) bool {
	if list == nil || id == "" {
		return false
	}
	items := *list
	for i := 0; i < len(items); i++ {
		if items[i].ID == id {
			items = append(items[:i], items[i+1:]...)
			*list = items
			return true
		}
		if len(items[i].Children) > 0 {
			if removeBulletByID(&items[i].Children, id) {
				items[i].Children = items[i].Children
				*list = items
				return true
			}
		}
	}
	return false
}
