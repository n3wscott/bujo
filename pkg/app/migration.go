package app

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
)

// MigrationCandidate represents an open task that needs attention during a migration session.
type MigrationCandidate struct {
	Entry       *entry.Entry
	Parent      *entry.Entry
	LastTouched time.Time
}

// MigrationCandidates returns open, completable entries that require review in a migration
// session. When the since window is zero, all open tasks/events outside the Future tree
// (and not scheduled on future daily collections) are returned alongside top-level Future
// entries. When a non-zero window is provided, candidates must have been touched within the
// window, with top-level Future entries always included. LastTouched reflects the most recent
// history record (or creation time).
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
	now := until
	if now.IsZero() {
		now = time.Now()
	}
	if until.IsZero() {
		until = now
	}
	sinceZero := since.IsZero()

	for _, e := range all {
		if !isCompletableCandidate(e) {
			continue
		}
		last := lastTouchedAt(e)
		if !until.IsZero() && !last.IsZero() && last.After(until) {
			continue
		}

		collectionPath := strings.TrimSpace(e.Collection)
		if collectionPath == "" {
			continue
		}
		topLevelFuture := isTopLevelFuture(collectionPath)
		if !topLevelFuture {
			if isInFutureTree(collectionPath) {
				continue
			}
			if isDailyCollectionAfter(now, collectionPath) {
				continue
			}
			if !sinceZero && (last.IsZero() || last.Before(since)) {
				continue
			}
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

func isCompletableCandidate(e *entry.Entry) bool {
	if e == nil || e.ID == "" {
		return false
	}
	if e.Immutable {
		return false
	}
	switch e.Bullet {
	case glyph.Task, glyph.Event:
		return true
	default:
		return false
	}
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

func isTopLevelFuture(path string) bool {
	return strings.EqualFold(strings.TrimSpace(path), "Future")
}

func isInFutureTree(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	if isTopLevelFuture(path) {
		return false
	}
	return strings.HasPrefix(path, "Future/")
}

func isDailyCollectionAfter(now time.Time, path string) bool {
	if path == "" {
		return false
	}
	segments := strings.Split(path, "/")
	if len(segments) == 0 {
		return false
	}
	last := strings.TrimSpace(segments[len(segments)-1])
	if !collection.IsDayName(last) {
		return false
	}
	loc := now.Location()
	day, err := time.ParseInLocation("January 2, 2006", last, loc)
	if err != nil {
		day, err = time.Parse("January 2, 2006", last)
		if err != nil {
			return false
		}
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	return day.After(today)
}
