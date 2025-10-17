// Package mcp provides the Model Context Protocol server integration for bujo.
package mcp

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/store"
)

// Service coordinates persistence-backed operations that are shared by the MCP server.
type Service struct {
	Persistence store.Persistence
}

// ErrEntryNotFound is returned when an entry cannot be located in persistence.
var ErrEntryNotFound = errors.New("entry not found")

// AddEntryOptions captures the parameters used to create a new entry.
type AddEntryOptions struct {
	Collection string
	Message    string
	Bullet     glyph.Bullet
	Signifier  glyph.Signifier
	On         *time.Time
}

// MoveEntryOptions captures the parameters required to migrate an entry.
type MoveEntryOptions struct {
	ID                string
	TargetCollection  string
	ReplacementBullet glyph.Bullet
}

// CollectionSummary describes a collection and basic aggregate metadata.
type CollectionSummary struct {
	Name             string `json:"name"`
	EntryCount       int    `json:"entryCount"`
	OpenCount        int    `json:"openCount"`
	LastUpdated      string `json:"lastUpdated,omitempty"`
	LatestEntryTitle string `json:"latestEntryTitle,omitempty"`
}

// EntryDTO is a transport-friendly projection of an entry.
type EntryDTO struct {
	ID                 string `json:"id"`
	Collection         string `json:"collection"`
	Message            string `json:"message,omitempty"`
	Bullet             string `json:"bullet"`
	BulletSymbol       string `json:"bulletSymbol"`
	BulletMeaning      string `json:"bulletMeaning"`
	Signifier          string `json:"signifier"`
	SignifierSymbol    string `json:"signifierSymbol"`
	SignifierMeaning   string `json:"signifierMeaning"`
	IsCompleted        bool   `json:"isCompleted"`
	IsIrrelevant       bool   `json:"isIrrelevant"`
	CreatedISO         string `json:"created"`
	ScheduledISO       string `json:"scheduled,omitempty"`
	CreatedUnix        int64  `json:"createdUnix"`
	ScheduledUnix      int64  `json:"scheduledUnix,omitempty"`
	SignifierPrintable bool   `json:"signifierPrintable"`
	BulletPrintable    bool   `json:"bulletPrintable"`
}

// NewService builds a service wrapper using the provided persistence layer.
func NewService(p store.Persistence) *Service {
	return &Service{Persistence: p}
}

// ListCollections returns summaries for every collection in persistence.
func (s *Service) ListCollections(ctx context.Context) ([]CollectionSummary, error) {
	if s.Persistence == nil {
		return nil, errors.New("persistence is not configured")
	}

	all := s.Persistence.MapAll(ctx)
	summaries := make([]CollectionSummary, 0, len(all))

	for name, entries := range all {
		if len(entries) == 0 {
			summaries = append(summaries, CollectionSummary{Name: name})
			continue
		}

		sortEntries(entries)

		openCount := 0
		for _, e := range entries {
			if e.Bullet != glyph.Completed && e.Bullet != glyph.Irrelevant {
				openCount++
			}
		}

		last := entries[len(entries)-1]
		summaries = append(summaries, CollectionSummary{
			Name:             name,
			EntryCount:       len(entries),
			OpenCount:        openCount,
			LastUpdated:      entry.FormatTime(last.Created.Time),
			LatestEntryTitle: last.Message,
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return strings.ToLower(summaries[i].Name) < strings.ToLower(summaries[j].Name)
	})

	return summaries, nil
}

// ListEntries gathers entries for the requested collection.
func (s *Service) ListEntries(ctx context.Context, collection string) ([]EntryDTO, error) {
	if s.Persistence == nil {
		return nil, errors.New("persistence is not configured")
	}
	if collection == "" {
		return nil, errors.New("collection is required")
	}

	entries := s.Persistence.List(ctx, collection)
	sortEntries(entries)
	return toDTOs(entries), nil
}

// ListAllEntries returns every entry persisted in the journal.
func (s *Service) ListAllEntries(ctx context.Context) ([]EntryDTO, error) {
	if s.Persistence == nil {
		return nil, errors.New("persistence is not configured")
	}

	entries := s.Persistence.ListAll(ctx)
	sortEntries(entries)
	return toDTOs(entries), nil
}

// AddEntry persists a new entry using the supplied options.
func (s *Service) AddEntry(ctx context.Context, opts AddEntryOptions) (*EntryDTO, error) {
	if s.Persistence == nil {
		return nil, errors.New("persistence is not configured")
	}
	if opts.Collection == "" {
		return nil, errors.New("collection is required")
	}

	if opts.Bullet == "" {
		opts.Bullet = glyph.Task
	}
	if opts.Signifier == "" {
		opts.Signifier = glyph.None
	}

	e := entry.New(opts.Collection, opts.Bullet, opts.Message)
	e.Signifier = opts.Signifier
	if opts.On != nil {
		e.On = &entry.Timestamp{Time: *opts.On}
	}

	if err := s.Persistence.Store(e); err != nil {
		return nil, err
	}

	dto := toDTO(e)
	return &dto, nil
}

// CompleteEntry marks an entry as completed.
func (s *Service) CompleteEntry(ctx context.Context, id string) (*EntryDTO, error) {
	e, err := s.findEntry(ctx, id)
	if err != nil {
		return nil, err
	}
	e.Complete()
	if err := s.Persistence.Store(e); err != nil {
		return nil, err
	}
	dto := toDTO(e)
	return &dto, nil
}

// StrikeEntry marks an entry as no longer relevant.
func (s *Service) StrikeEntry(ctx context.Context, id string) (*EntryDTO, error) {
	e, err := s.findEntry(ctx, id)
	if err != nil {
		return nil, err
	}
	e.Strike()
	if err := s.Persistence.Store(e); err != nil {
		return nil, err
	}
	dto := toDTO(e)
	return &dto, nil
}

// MoveEntry migrates an entry to another collection while swapping the original bullet.
func (s *Service) MoveEntry(ctx context.Context, opts MoveEntryOptions) (*EntryDTO, *EntryDTO, error) {
	if s.Persistence == nil {
		return nil, nil, errors.New("persistence is not configured")
	}
	if opts.ID == "" || opts.TargetCollection == "" {
		return nil, nil, errors.New("id and target collection are required")
	}
	if opts.ReplacementBullet == "" {
		opts.ReplacementBullet = glyph.MovedCollection
	}

	e, err := s.findEntry(ctx, opts.ID)
	if err != nil {
		return nil, nil, err
	}

	newEntry := e.Move(opts.ReplacementBullet, opts.TargetCollection)
	if err := s.Persistence.Store(e); err != nil {
		return nil, nil, err
	}
	if err := s.Persistence.Store(newEntry); err != nil {
		return nil, nil, err
	}

	updated := toDTO(e)
	clone := toDTO(newEntry)
	return &updated, &clone, nil
}

// UpdateBullet changes the bullet associated with an entry.
func (s *Service) UpdateBullet(ctx context.Context, id string, bullet glyph.Bullet) (*EntryDTO, error) {
	if bullet == "" {
		return nil, errors.New("bullet is required")
	}

	e, err := s.findEntry(ctx, id)
	if err != nil {
		return nil, err
	}
	e.Bullet = bullet
	if err := s.Persistence.Store(e); err != nil {
		return nil, err
	}
	dto := toDTO(e)
	return &dto, nil
}

// UpdateSignifier applies a new signifier to the entry.
func (s *Service) UpdateSignifier(ctx context.Context, id string, signifier glyph.Signifier) (*EntryDTO, error) {
	e, err := s.findEntry(ctx, id)
	if err != nil {
		return nil, err
	}
	e.Signifier = signifier
	if err := s.Persistence.Store(e); err != nil {
		return nil, err
	}
	dto := toDTO(e)
	return &dto, nil
}

// UpdateMessage rewrites the entry message.
func (s *Service) UpdateMessage(ctx context.Context, id, message string) (*EntryDTO, error) {
	e, err := s.findEntry(ctx, id)
	if err != nil {
		return nil, err
	}
	e.Message = message
	if err := s.Persistence.Store(e); err != nil {
		return nil, err
	}
	dto := toDTO(e)
	return &dto, nil
}

// SearchEntries performs a substring match across collection names and messages.
func (s *Service) SearchEntries(ctx context.Context, query string, limit int) ([]EntryDTO, error) {
	if s.Persistence == nil {
		return nil, errors.New("persistence is not configured")
	}
	q := strings.TrimSpace(strings.ToLower(query))
	if q == "" {
		return []EntryDTO{}, nil
	}
	if limit <= 0 {
		limit = 20
	}

	all := s.Persistence.ListAll(ctx)
	sortEntries(all)

	results := make([]EntryDTO, 0, limit)
	for _, e := range all {
		if len(results) >= limit {
			break
		}
		if strings.Contains(strings.ToLower(e.Collection), q) || strings.Contains(strings.ToLower(e.Message), q) {
			results = append(results, toDTO(e))
		}
	}
	return results, nil
}

// EntryByID locates an entry by id and returns the DTO representation.
func (s *Service) EntryByID(ctx context.Context, id string) (*EntryDTO, error) {
	e, err := s.findEntry(ctx, id)
	if err != nil {
		return nil, err
	}
	dto := toDTO(e)
	return &dto, nil
}

func (s *Service) findEntry(ctx context.Context, id string) (*entry.Entry, error) {
	if s.Persistence == nil {
		return nil, errors.New("persistence is not configured")
	}
	if id == "" {
		return nil, errors.New("id is required")
	}

	all := s.Persistence.ListAll(ctx)
	for _, e := range all {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrEntryNotFound, id)
}

func sortEntries(entries []*entry.Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		a := entries[i].Created.Time
		b := entries[j].Created.Time
		if !a.Equal(b) {
			return a.Before(b)
		}
		return strings.ToLower(entries[i].ID) < strings.ToLower(entries[j].ID)
	})
}

func toDTOs(entries []*entry.Entry) []EntryDTO {
	out := make([]EntryDTO, 0, len(entries))
	for _, e := range entries {
		out = append(out, toDTO(e))
	}
	return out
}

func toDTO(e *entry.Entry) EntryDTO {
	bGlyph := e.Bullet.Glyph()
	sGlyph := e.Signifier.Glyph()

	dto := EntryDTO{
		ID:                 e.ID,
		Collection:         e.Collection,
		Message:            e.Message,
		Bullet:             string(e.Bullet),
		BulletSymbol:       bGlyph.Symbol,
		BulletMeaning:      bGlyph.Meaning,
		BulletPrintable:    bGlyph.Printed,
		Signifier:          string(e.Signifier),
		SignifierSymbol:    sGlyph.Symbol,
		SignifierMeaning:   sGlyph.Meaning,
		SignifierPrintable: sGlyph.Printed,
		IsCompleted:        e.Bullet == glyph.Completed,
		IsIrrelevant:       e.Bullet == glyph.Irrelevant,
		CreatedISO:         entry.FormatTime(e.Created.Time),
		CreatedUnix:        e.Created.Unix(),
	}
	if e.On != nil && !e.On.IsZero() {
		dto.ScheduledISO = entry.FormatTime(e.On.Time)
		dto.ScheduledUnix = e.On.Unix()
	}
	return dto
}

// ParseBullet attempts to resolve a bullet identifier or alias to a glyph.
func ParseBullet(input string, fallback glyph.Bullet) (glyph.Bullet, error) {
	if input == "" {
		if fallback == "" {
			return glyph.Task, nil
		}
		return fallback, nil
	}

	lower := strings.ToLower(strings.TrimSpace(input))

	for k, g := range glyph.DefaultBullets() {
		if strings.EqualFold(string(k), lower) || strings.EqualFold(g.Meaning, lower) {
			return k, nil
		}
		for _, alias := range g.Aliases {
			if strings.EqualFold(alias, lower) {
				return k, nil
			}
		}
	}

	if b, err := glyph.BulletForAlias(lower); err == nil && b != glyph.Any {
		return b, nil
	}

	if fallback != "" {
		return fallback, fmt.Errorf("unknown bullet %q", input)
	}
	return "", fmt.Errorf("unknown bullet %q", input)
}

// ParseSignifier resolves a signifier identifier or alias.
func ParseSignifier(input string, fallback glyph.Signifier) (glyph.Signifier, error) {
	if input == "" {
		if fallback == "" {
			return glyph.None, nil
		}
		return fallback, nil
	}

	lower := strings.ToLower(strings.TrimSpace(input))
	for k, g := range glyph.DefaultSignifiers() {
		if strings.EqualFold(string(k), lower) || strings.EqualFold(g.Meaning, lower) {
			return k, nil
		}
		for _, alias := range g.Aliases {
			if strings.EqualFold(alias, lower) {
				return k, nil
			}
		}
	}

	switch lower {
	case "", "none":
		return glyph.None, nil
	}

	if fallback != "" {
		return fallback, fmt.Errorf("unknown signifier %q", input)
	}
	return "", fmt.Errorf("unknown signifier %q", input)
}
