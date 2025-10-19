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
	Completed   bool
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

	byID := indexEntriesByID(all)
	grouped := make(map[string]map[string]*ReportItem)
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
		item := ensureReportItem(grouped, e.Collection, e)
		item.Completed = true
		if completedAt.After(item.CompletedAt) {
			item.CompletedAt = completedAt
		}
		total++

		parentID := e.ParentID
		visited := make(map[string]bool)
		for parentID != "" {
			if visited[parentID] {
				break
			}
			visited[parentID] = true
			parent := byID[parentID]
			if parent == nil {
				break
			}
			ensureReportItem(grouped, parent.Collection, parent)
			parentID = parent.ParentID
		}
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
		items := make([]ReportItem, 0, len(grouped[collection]))
		for _, e := range all {
			if e == nil || e.Collection != collection {
				continue
			}
			if item, ok := grouped[collection][e.ID]; ok && item != nil {
				items = append(items, *item)
			}
		}
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

func ensureReportItem(grouped map[string]map[string]*ReportItem, collection string, e *entry.Entry) *ReportItem {
	if e == nil {
		return nil
	}
	bucket, ok := grouped[collection]
	if !ok {
		bucket = make(map[string]*ReportItem)
		grouped[collection] = bucket
	}
	if item, ok := bucket[e.ID]; ok && item != nil {
		return item
	}
	item := &ReportItem{Entry: e}
	bucket[e.ID] = item
	return item
}
