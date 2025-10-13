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
			e.Bullet = b
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
	return s.SetBullet(ctx, id, glyph.Completed)
}

// Strike marks an entry irrelevant (strike-through semantics).
func (s *Service) Strike(ctx context.Context, id string) (*entry.Entry, error) {
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	for _, e := range s.Persistence.ListAll(ctx) {
		if e.ID == id {
			e.Strike()
			if err := s.Persistence.Store(e); err != nil {
				return nil, err
			}
			return e, nil
		}
	}
	return nil, errors.New("app: entry not found")
}

// Move clones an entry into the target collection and marks the original as moved.
func (s *Service) Move(ctx context.Context, id string, target string) (*entry.Entry, error) {
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}
	for _, e := range s.Persistence.ListAll(ctx) {
		if e.ID == id {
			// Decide moved glyph based on direction
			moved := glyph.MovedCollection
			if strings.EqualFold(target, "future") {
				moved = glyph.MovedFuture
			}

			ne := e.Move(moved, target)
			// Store new entry first, then update original
			if err := s.Persistence.Store(ne); err != nil {
				return nil, err
			}
			if err := s.Persistence.Store(e); err != nil {
				return nil, err
			}
			return ne, nil
		}
	}
	return nil, errors.New("app: entry not found")
}
