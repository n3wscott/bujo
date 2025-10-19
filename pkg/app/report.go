package app

import (
	"context"
	"sort"
	"time"

	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
)

// ReportItem captures a completed entry and the timestamp it was completed.
type ReportItem struct {
	Entry       *entry.Entry
	CompletedAt time.Time
}

// ReportSection groups completed entries by collection.
type ReportSection struct {
	Collection string
	Entries    []ReportItem
}

// ReportResult encapsulates a completed-entries report for a time window.
type ReportResult struct {
	Since    time.Time
	Until    time.Time
	Sections []ReportSection
	Total    int
}

// Report returns completed entries grouped by collection between the provided bounds.
func (s *Service) Report(ctx context.Context, since, until time.Time) (ReportResult, error) {
	if since.After(until) {
		since, until = until, since
	}
	all, err := s.listAll(ctx)
	if err != nil {
		return ReportResult{}, err
	}

	grouped := make(map[string][]ReportItem)
	total := 0
	for _, e := range all {
		if e == nil {
			continue
		}
		if e.Bullet != glyph.Completed {
			continue
		}
		completedAt, ok := e.LastCompletionTime()
		if !ok {
			continue
		}
		if completedAt.Before(since) || completedAt.After(until) {
			continue
		}
		grouped[e.Collection] = append(grouped[e.Collection], ReportItem{
			Entry:       e,
			CompletedAt: completedAt,
		})
		total++
	}

	if len(grouped) == 0 {
		return ReportResult{
			Since: since,
			Until: until,
		}, nil
	}

	collections := make([]string, 0, len(grouped))
	for collection := range grouped {
		collections = append(collections, collection)
	}
	sort.Strings(collections)

	sections := make([]ReportSection, 0, len(collections))
	for _, collection := range collections {
		items := grouped[collection]
		sort.SliceStable(items, func(i, j int) bool {
			left := items[i].CompletedAt
			right := items[j].CompletedAt
			if left.Equal(right) {
				li := items[i].Entry
				ri := items[j].Entry
				return li.ID < ri.ID
			}
			return left.After(right)
		})
		sections = append(sections, ReportSection{
			Collection: collection,
			Entries:    items,
		})
	}

	return ReportResult{
		Since:    since,
		Until:    until,
		Sections: sections,
		Total:    total,
	}, nil
}
