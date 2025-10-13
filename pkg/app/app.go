package app

import (
	"context"
	"errors"
	"sort"
	"strings"

	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/store"
)

// Service provides high-level operations for entries and collections.
// It wraps persistence and entry transformations so UIs and CLIs can share logic.
type Service struct {
	Persistence store.Persistence
}

var ErrImmutable = errors.New("app: entry is immutable")

// Collections returns sorted collection names.
func (s *Service) Collections(ctx context.Context) ([]string, error) {
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	cols := s.Persistence.Collections(ctx, "")
	sort.Strings(cols)
	return cols, nil
}

// Entries lists entries for a collection.
func (s *Service) Entries(ctx context.Context, collection string) ([]*entry.Entry, error) {
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	return s.Persistence.List(ctx, collection), nil
}

// Watch subscribes to persistence change events.
func (s *Service) Watch(ctx context.Context) (<-chan store.Event, error) {
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	return s.Persistence.Watch(ctx)
}

// Add creates and stores a new entry.
func (s *Service) Add(ctx context.Context, collection string, b glyph.Bullet, msg string, sig glyph.Signifier) (*entry.Entry, error) {
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
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	for _, e := range s.Persistence.ListAll(ctx) {
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
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	for _, e := range s.Persistence.ListAll(ctx) {
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
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	for _, e := range s.Persistence.ListAll(ctx) {
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
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	for _, e := range s.Persistence.ListAll(ctx) {
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
	if s.Persistence == nil {
		return errors.New("app: no persistence configured")
	}
	for _, e := range s.Persistence.ListAll(ctx) {
		if e.ID == id {
			return s.Persistence.Delete(e)
		}
	}
	return errors.New("app: entry not found")
}

// Complete marks an entry completed.
func (s *Service) Complete(ctx context.Context, id string) (*entry.Entry, error) {
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	all := s.Persistence.ListAll(ctx)
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
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	all := s.Persistence.ListAll(ctx)
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
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	all := s.Persistence.ListAll(ctx)
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
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	all := s.Persistence.ListAll(ctx)
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
	if s.Persistence == nil {
		return errors.New("app: no persistence configured")
	}
	return s.Persistence.EnsureCollection(collection)
}

// EnsureCollections ensures each collection in the slice exists.
func (s *Service) EnsureCollections(ctx context.Context, collections []string) error {
	for _, name := range collections {
		if err := s.EnsureCollection(ctx, name); err != nil {
			return err
		}
	}
	return nil
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

func ensureMutable(e *entry.Entry) error {
	if e == nil {
		return errors.New("app: entry not found")
	}
	if e.Immutable {
		return ErrImmutable
	}
	return nil
}
