package app

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
)

// MigrationCandidate represents an open task that needs attention during a migration session.
type MigrationCandidate struct {
	Entry       *entry.Entry
	Parent      *entry.Entry
	LastTouched time.Time
}

// MigrationCandidates returns open tasks that were touched within the provided window.
// A task is considered "open" when it still carries the task bullet (i.e. not completed,
// moved, struck, or converted to another bullet). The LastTouched timestamp reflects the
// most recent history record (or creation time).
func (s *Service) MigrationCandidates(ctx context.Context, since, until time.Time) ([]MigrationCandidate, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if s.Persistence == nil {
		return nil, errors.New("app: no persistence configured")
	}

	all, err := s.listAll(ctx)
	if err != nil {
		return nil, err
	}
	items := indexEntriesByID(all)

	results := make([]MigrationCandidate, 0, len(all))
	for _, e := range all {
		if e == nil || e.ID == "" {
			continue
		}
		if !isOpenTask(e) {
			continue
		}
		last := lastTouchedAt(e)
		if last.Before(since) || last.After(until) {
			continue
		}
		var parent *entry.Entry
		if e.ParentID != "" {
			parent = items[e.ParentID]
		}
		results = append(results, MigrationCandidate{
			Entry:       e,
			Parent:      parent,
			LastTouched: last,
		})
	}

	sort.SliceStable(results, func(i, j int) bool {
		li := results[i].LastTouched
		lj := results[j].LastTouched
		if li.Equal(lj) {
			return strings.Compare(results[i].Entry.Collection, results[j].Entry.Collection) < 0
		}
		return li.After(lj)
	})

	return results, nil
}

func isOpenTask(e *entry.Entry) bool {
	if e == nil {
		return false
	}
	if e.Bullet != glyph.Task {
		return false
	}
	if strings.TrimSpace(e.Message) == "" && e.ParentID == "" {
		// Allow blank tasks, but keep logic symmetrical in case we need special handling later.
		return true
	}
	return true
}

func lastTouchedAt(e *entry.Entry) time.Time {
	if e == nil {
		return time.Time{}
	}
	latest := e.Created.Time
	for _, record := range e.History {
		ts := record.Timestamp.Time
		if ts.IsZero() {
			continue
		}
		if latest.IsZero() || ts.After(latest) {
			latest = ts
		}
	}
	return latest
}
