// Package app exposes shared services for manipulating bujo entries and collections.
package app

import (
	"context"
	"errors"
	"sort"
	"strings"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/store"
)

// Service provides high-level operations for entries and collections.
// It wraps persistence and entry transformations so UIs and CLIs can share logic.
type Service struct {
	Persistence store.Persistence
}

// ErrImmutable indicates operations on an immutable entry are not allowed.
var ErrImmutable = errors.New("app: entry is immutable")

// ErrInvalidCollection indicates a collection path or segment was invalid.
var ErrInvalidCollection = errors.New("app: invalid collection")

// Collections returns sorted collection names.
func (s *Service) Collections(ctx context.Context) ([]string, error) {
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	cols := s.Persistence.Collections(ctx, "")
	sort.Strings(cols)
	return cols, nil
}

// CollectionsMeta returns collection metadata filtered by prefix.
func (s *Service) CollectionsMeta(ctx context.Context, prefix string) ([]collection.Meta, error) {
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	metas := s.Persistence.CollectionsMeta(ctx, prefix)
	return metas, nil
}

// Entries lists entries for a collection.
func (s *Service) Entries(ctx context.Context, collection string) ([]*entry.Entry, error) {
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	list := s.Persistence.List(ctx, collection)
	if err := s.ensureMovedImmutable(ctx, list); err != nil {
		return nil, err
	}
	return list, nil
}

// Watch subscribes to persistence change events.
func (s *Service) Watch(ctx context.Context) (<-chan store.Event, error) {
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	return s.Persistence.Watch(ctx)
}

// Add creates and stores a new entry.
func (s *Service) Add(_ context.Context, collection string, b glyph.Bullet, msg string, sig glyph.Signifier) (*entry.Entry, error) {
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	e := entry.New(collection, b, msg)
	e.Signifier = sig
	if err := s.Persistence.Store(e); err != nil {
		return nil, err
	}
	return e, nil
}

// Edit updates the message for the entry with the given id.
func (s *Service) Edit(ctx context.Context, id string, newMsg string) (*entry.Entry, error) {
	all, err := s.listAll(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range all {
		if e.ID == id {
			if err := ensureMutable(e); err != nil {
				return nil, err
			}
			e.Message = newMsg
			if err := s.Persistence.Store(e); err != nil {
				return nil, err
			}
			return e, nil
		}
	}
	return nil, errors.New("app: entry not found")
}

// SetBullet sets the bullet type for the entry id.
func (s *Service) SetBullet(ctx context.Context, id string, b glyph.Bullet) (*entry.Entry, error) {
	all, err := s.listAll(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range all {
		if e.ID == id {
			if err := ensureMutable(e); err != nil {
				return nil, err
			}
			e.Bullet = b
			if err := s.Persistence.Store(e); err != nil {
				return nil, err
			}
			return e, nil
		}
	}
	return nil, errors.New("app: entry not found")
}

// SetSignifier assigns the provided signifier to the entry id.
func (s *Service) SetSignifier(ctx context.Context, id string, sig glyph.Signifier) (*entry.Entry, error) {
	all, err := s.listAll(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range all {
		if e.ID == id {
			if err := ensureMutable(e); err != nil {
				return nil, err
			}
			e.Signifier = sig
			if err := s.Persistence.Store(e); err != nil {
				return nil, err
			}
			return e, nil
		}
	}
	return nil, errors.New("app: entry not found")
}

// ToggleSignifier toggles a given signifier on/off for the entry id. If the
// same signifier is set, it clears it; otherwise it sets it.
func (s *Service) ToggleSignifier(ctx context.Context, id string, sig glyph.Signifier) (*entry.Entry, error) {
	all, err := s.listAll(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range all {
		if e.ID == id {
			if err := ensureMutable(e); err != nil {
				return nil, err
			}
			if e.Signifier == sig {
				e.Signifier = glyph.None
			} else {
				e.Signifier = sig
			}
			if err := s.Persistence.Store(e); err != nil {
				return nil, err
			}
			return e, nil
		}
	}
	return nil, errors.New("app: entry not found")
}

// Delete removes an entry permanently.
func (s *Service) Delete(ctx context.Context, id string) error {
	all, err := s.listAll(ctx)
	if err != nil {
		return err
	}
	for _, e := range all {
		if e.ID == id {
			return s.Persistence.Delete(e)
		}
	}
	return errors.New("app: entry not found")
}

// Complete marks an entry completed.
func (s *Service) Complete(ctx context.Context, id string) (*entry.Entry, error) {
	all, err := s.listAll(ctx)
	if err != nil {
		return nil, err
	}
	items := indexEntriesByID(all)
	e, ok := items[id]
	if !ok {
		return nil, errors.New("app: entry not found")
	}
	if err := ensureMutable(e); err != nil {
		return nil, err
	}
	for childID := range collectSubtreeIDs(items, id) {
		if childID == id {
			continue
		}
		items[childID].ParentID = e.ParentID
		if err := s.Persistence.Store(items[childID]); err != nil {
			return nil, err
		}
	}
	e.Complete()
	if err := s.Persistence.Store(e); err != nil {
		return nil, err
	}
	return e, nil
}

// Strike marks an entry irrelevant (strike-through semantics).
func (s *Service) Strike(ctx context.Context, id string) (*entry.Entry, error) {
	all, err := s.listAll(ctx)
	if err != nil {
		return nil, err
	}
	items := indexEntriesByID(all)
	e, ok := items[id]
	if !ok {
		return nil, errors.New("app: entry not found")
	}
	if err := ensureMutable(e); err != nil {
		return nil, err
	}
	for childID := range collectSubtreeIDs(items, id) {
		if childID == id {
			continue
		}
		items[childID].ParentID = e.ParentID
		if err := s.Persistence.Store(items[childID]); err != nil {
			return nil, err
		}
	}
	e.Strike()
	if err := s.Persistence.Store(e); err != nil {
		return nil, err
	}
	return e, nil
}

// Move clones an entry into the target collection and marks the original as moved.
func (s *Service) Move(ctx context.Context, id string, target string) (*entry.Entry, error) {
	all, err := s.listAll(ctx)
	if err != nil {
		return nil, err
	}
	items := indexEntriesByID(all)
	root, ok := items[id]
	if !ok {
		return nil, errors.New("app: entry not found")
	}
	if err := ensureMutable(root); err != nil {
		return nil, err
	}
	if strings.EqualFold(strings.TrimSpace(root.Collection), strings.TrimSpace(target)) {
		return root, nil
	}
	moved := glyph.MovedCollection
	if strings.EqualFold(target, "future") {
		moved = glyph.MovedFuture
	}
	subtree := collectSubtree(items, id)
	if len(subtree) == 0 {
		return nil, errors.New("app: entry not found")
	}
	for _, node := range subtree {
		if node.Immutable {
			return nil, ErrImmutable
		}
	}
	mapping := make(map[string]*entry.Entry, len(subtree))
	var cloneRoot *entry.Entry
	for _, node := range subtree {
		if node.ID == id {
			clone := node.Move(moved, target)
			clone.ParentID = ""
			if err := s.Persistence.Store(clone); err != nil {
				return nil, err
			}
			if err := s.Persistence.Store(node); err != nil {
				return nil, err
			}
			mapping[node.ID] = clone
			cloneRoot = clone
			continue
		}
		oldParent := node.ParentID
		clone := node.Move(moved, target)
		if parentClone, ok := mapping[oldParent]; ok {
			clone.ParentID = parentClone.ID
		} else {
			clone.ParentID = ""
		}
		if err := s.Persistence.Store(clone); err != nil {
			return nil, err
		}
		if err := s.Persistence.Store(node); err != nil {
			return nil, err
		}
		mapping[node.ID] = clone
	}
	return cloneRoot, nil
}

// SetParent reassigns the parent relationship for an entry within a collection.
func (s *Service) SetParent(ctx context.Context, id, parentID string) (*entry.Entry, error) {
	all, err := s.listAll(ctx)
	if err != nil {
		return nil, err
	}
	items := indexEntriesByID(all)
	child, ok := items[id]
	if !ok {
		return nil, errors.New("app: entry not found")
	}
	if err := ensureMutable(child); err != nil {
		return nil, err
	}
	if parentID == "" {
		child.ParentID = ""
		if err := s.Persistence.Store(child); err != nil {
			return nil, err
		}
		return child, nil
	}
	parent, ok := items[parentID]
	if !ok {
		return nil, errors.New("app: parent entry not found")
	}
	if parent.Collection != child.Collection {
		return nil, errors.New("app: parent must be in same collection")
	}
	if createsCycle(items, id, parentID) {
		return nil, errors.New("app: parent assignment would create cycle")
	}
	if err := ensureMutable(parent); err != nil {
		return nil, err
	}
	child.ParentID = parentID
	if err := s.Persistence.Store(child); err != nil {
		return nil, err
	}
	return child, nil
}

// EnsureCollection ensures the named collection exists even if empty.
func (s *Service) EnsureCollection(ctx context.Context, collection string) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if s.Persistence == nil {
		return errors.New("app: no persistence configured")
	}
	return s.Persistence.EnsureCollection(collection)
}

// EnsureCollections ensures each collection in the slice exists.
func (s *Service) EnsureCollections(ctx context.Context, collections []string) error {
	if s.Persistence == nil {
		return errors.New("app: no persistence configured")
	}
	metas, err := s.CollectionsMeta(ctx, "")
	if err != nil {
		return err
	}
	typeMap := make(map[string]collection.Type, len(metas))
	for _, meta := range metas {
		if meta.Type == "" {
			meta.Type = collection.TypeGeneric
		}
		typeMap[meta.Name] = meta.Type
	}
	children := buildChildrenIndex(metas)

	existingNames := make([]string, 0, len(typeMap))
	for name := range typeMap {
		existingNames = append(existingNames, name)
	}
	sort.SliceStable(existingNames, func(i, j int) bool {
		return strings.Count(existingNames[i], "/") < strings.Count(existingNames[j], "/")
	})
	for _, name := range existingNames {
		parentType := collection.TypeGeneric
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			parent := name[:idx]
			if typ, ok := typeMap[parent]; ok {
				parentType = typ
			}
		}
		current := typeMap[name]
		inferred := inferCollectionTypeFromContext(name, parentType, current, children[name])
		if inferred != current {
			if err := s.Persistence.SetCollectionType(name, inferred); err != nil {
				return err
			}
			typeMap[name] = inferred
		}
	}

	paths := append([]string(nil), collections...)
	sort.SliceStable(paths, func(i, j int) bool {
		return strings.Count(paths[i], "/") < strings.Count(paths[j], "/")
	})

	for _, full := range paths {
		trimmed := strings.TrimSpace(full)
		if trimmed == "" {
			return ErrInvalidCollection
		}
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return err
			}
		}

		segments := strings.Split(trimmed, "/")
		childName := segments[len(segments)-1]
		parentPath := ""
		if len(segments) > 1 {
			parentPath = strings.Join(segments[:len(segments)-1], "/")
		}

		parentType := collection.TypeGeneric
		parentName := ""
		if parentPath != "" {
			if typ, ok := typeMap[parentPath]; ok {
				parentType = typ
			}
			parentSegments := strings.Split(parentPath, "/")
			parentName = parentSegments[len(parentSegments)-1]
		}

		if parentType != collection.TypeGeneric {
			if err := collection.ValidateChildName(parentType, parentName, childName); err != nil {
				return err
			}
		}

		existing, exists := typeMap[trimmed]
		if exists && existing == collection.TypeGeneric {
			candidate := collection.GuessType(childName, parentType)
			candidate = preferCollectionType(candidate, trimmed, parentPath, childName, parentType)
			if candidate != existing {
				if err := s.Persistence.SetCollectionType(trimmed, candidate); err != nil {
					return err
				}
				typeMap[trimmed] = candidate
				existing = candidate
			}
		}
		if !exists {
			typ := collection.GuessType(childName, parentType)
			typ = preferCollectionType(typ, trimmed, parentPath, childName, parentType)
			if err := s.Persistence.EnsureCollectionTyped(trimmed, typ); err != nil {
				return err
			}
			typeMap[trimmed] = typ
			existing = typ
		}
		children[parentPath] = appendUniqueChild(children[parentPath], childName)
		if parentPath != "" {
			if parentCurr, ok := typeMap[parentPath]; ok {
				grandParentType := collection.TypeGeneric
				if idx := strings.LastIndex(parentPath, "/"); idx >= 0 {
					grandParentPath := parentPath[:idx]
					if grandParentTypeCandidate, ok := typeMap[grandParentPath]; ok {
						grandParentType = grandParentTypeCandidate
					}
				}
				inferred := inferCollectionTypeFromContext(parentPath, grandParentType, parentCurr, children[parentPath])
				if inferred != parentCurr {
					if err := s.Persistence.SetCollectionType(parentPath, inferred); err != nil {
						return err
					}
					typeMap[parentPath] = inferred
				}
			}
		}
	}
	return nil
}

// EnsureCollectionOfType ensures the collection exists and records its type.
func (s *Service) EnsureCollectionOfType(ctx context.Context, collectionName string, typ collection.Type) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if s.Persistence == nil {
		return errors.New("app: no persistence configured")
	}
	segments := strings.Split(strings.TrimSpace(collectionName), "/")
	var paths []string
	var parts []string
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		parts = append(parts, segment)
		path := strings.Join(parts, "/")
		if len(paths) == 0 || paths[len(paths)-1] != path {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return ErrInvalidCollection
	}
	if err := s.EnsureCollections(ctx, paths); err != nil {
		return err
	}
	return s.SetCollectionType(ctx, collectionName, typ)
}

// SetCollectionType updates metadata for an existing collection.
func (s *Service) SetCollectionType(ctx context.Context, collectionName string, typ collection.Type) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if s.Persistence == nil {
		return errors.New("app: no persistence configured")
	}
	if strings.TrimSpace(collectionName) == "" {
		return ErrInvalidCollection
	}
	metas := s.Persistence.CollectionsMeta(ctx, "")
	typeMap := make(map[string]collection.Type, len(metas))
	for _, meta := range metas {
		if meta.Type == "" {
			meta.Type = collection.TypeGeneric
		}
		typeMap[meta.Name] = meta.Type
	}
	current, ok := typeMap[collectionName]
	if !ok {
		if err := s.Persistence.EnsureCollectionTyped(collectionName, typ); err != nil {
			return err
		}
		return nil
	}
	if err := collection.ValidateTypeTransition(current, typ); err != nil {
		return err
	}
	children := buildChildrenIndex(metas)[collectionName]
	if typ != collection.TypeGeneric {
		parentLabel := lastSegment(collectionName)
		for _, child := range children {
			if err := collection.ValidateChildName(typ, parentLabel, child); err != nil {
				return err
			}
		}
	}
	return s.Persistence.SetCollectionType(collectionName, typ)
}

// Lock marks an entry immutable.
func (s *Service) Lock(ctx context.Context, id string) (*entry.Entry, error) {
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	for _, e := range s.Persistence.ListAll(ctx) {
		if e.ID == id {
			e.Lock()
			if err := s.Persistence.Store(e); err != nil {
				return nil, err
			}
			return e, nil
		}
	}
	return nil, errors.New("app: entry not found")
}

// Unlock clears the immutable flag.
func (s *Service) Unlock(ctx context.Context, id string) (*entry.Entry, error) {
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	for _, e := range s.Persistence.ListAll(ctx) {
		if e.ID == id {
			e.Unlock()
			if err := s.Persistence.Store(e); err != nil {
				return nil, err
			}
			return e, nil
		}
	}
	return nil, errors.New("app: entry not found")
}

func indexEntriesByID(entries []*entry.Entry) map[string]*entry.Entry {
	indexed := make(map[string]*entry.Entry, len(entries))
	for _, e := range entries {
		if e == nil || e.ID == "" {
			continue
		}
		indexed[e.ID] = e
	}
	return indexed
}

func collectSubtree(items map[string]*entry.Entry, rootID string) []*entry.Entry {
	order := make([]*entry.Entry, 0)
	visited := make(map[string]bool)
	var visit func(string)
	visit = func(id string) {
		if visited[id] {
			return
		}
		node, ok := items[id]
		if !ok {
			return
		}
		visited[id] = true
		order = append(order, node)
		for childID, child := range items {
			if child.ParentID == id {
				visit(childID)
			}
		}
	}
	visit(rootID)
	return order
}

func createsCycle(items map[string]*entry.Entry, childID, candidateParentID string) bool {
	current := candidateParentID
	for current != "" {
		if current == childID {
			return true
		}
		next := items[current]
		if next == nil {
			break
		}
		current = next.ParentID
	}
	return false
}

func collectSubtreeIDs(items map[string]*entry.Entry, rootID string) map[string]struct{} {
	result := make(map[string]struct{})
	var visit func(string)
	visit = func(id string) {
		if _, seen := result[id]; seen {
			return
		}
		result[id] = struct{}{}
		for childID, node := range items {
			if node.ParentID == id {
				visit(childID)
			}
		}
	}
	visit(rootID)
	return result
}

func buildChildrenIndex(metas []collection.Meta) map[string][]string {
	index := make(map[string]map[string]struct{})
	for _, meta := range metas {
		if meta.Name == "" {
			continue
		}
		parts := strings.Split(meta.Name, "/")
		for i := 1; i < len(parts); i++ {
			parent := strings.Join(parts[:i], "/")
			child := parts[i]
			if _, ok := index[parent]; !ok {
				index[parent] = make(map[string]struct{})
			}
			index[parent][child] = struct{}{}
		}
	}
	children := make(map[string][]string, len(index))
	for parent, set := range index {
		names := make([]string, 0, len(set))
		for child := range set {
			names = append(names, child)
		}
		sort.Strings(names)
		children[parent] = names
	}
	return children
}

func appendUniqueChild(children []string, child string) []string {
	for _, existing := range children {
		if existing == child {
			return children
		}
	}
	return append(children, child)
}

func preferCollectionType(base collection.Type, name, parentPath, childName string, parentType collection.Type) collection.Type {
	trimmed := strings.TrimSpace(name)
	child := strings.TrimSpace(childName)
	if parentPath == "" {
		if strings.EqualFold(trimmed, "Future") {
			return collection.TypeMonthly
		}
		if collection.IsMonthName(trimmed) {
			return collection.TypeDaily
		}
	}
	if strings.EqualFold(parentPath, "Future") {
		if collection.IsMonthName(child) {
			return collection.TypeDaily
		}
		return collection.TypeGeneric
	}
	if parentType == collection.TypeMonthly && collection.IsMonthName(child) {
		return collection.TypeDaily
	}
	return base
}

func lastSegment(path string) string {
	name := strings.TrimSpace(path)
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

func childrenMatchType(parentType collection.Type, parentLabel string, children []string) bool {
	if len(children) == 0 {
		return false
	}
	for _, child := range children {
		if err := collection.ValidateChildName(parentType, parentLabel, child); err != nil {
			return false
		}
	}
	return true
}

func inferCollectionTypeFromContext(name string, parentType collection.Type, current collection.Type, children []string) collection.Type {
	if current != collection.TypeGeneric {
		return current
	}
	label := lastSegment(name)
	if name == "Future" {
		if len(children) == 0 || childrenMatchType(collection.TypeMonthly, label, children) {
			return collection.TypeMonthly
		}
	}
	if parentType == collection.TypeMonthly {
		return collection.TypeDaily
	}
	if len(children) > 0 && childrenMatchType(collection.TypeMonthly, label, children) {
		return collection.TypeMonthly
	}
	if collection.IsMonthName(label) {
		if len(children) == 0 {
			return collection.TypeDaily
		}
		if childrenMatchType(collection.TypeDaily, label, children) {
			return collection.TypeDaily
		}
	}
	return current
}

func ensureMutable(e *entry.Entry) error {
	if e == nil {
		return errors.New("app: entry not found")
	}
	if e.Immutable {
		return ErrImmutable
	}
	return nil
}

func (s *Service) listAll(ctx context.Context) ([]*entry.Entry, error) {
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	all := s.Persistence.ListAll(ctx)
	if err := s.ensureMovedImmutable(ctx, all); err != nil {
		return nil, err
	}
	return all, nil
}

func (s *Service) ensureMovedImmutable(ctx context.Context, entries []*entry.Entry) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if s.Persistence == nil {
		return errors.New("app: no persistence configured")
	}
	for _, e := range entries {
		if e == nil {
			continue
		}
		if isMovedBullet(e.Bullet) && !e.Immutable {
			e.Immutable = true
			if err := s.Persistence.Store(e); err != nil {
				return err
			}
		}
	}
	return nil
}

func isMovedBullet(b glyph.Bullet) bool {
	return b == glyph.MovedCollection || b == glyph.MovedFuture
}
