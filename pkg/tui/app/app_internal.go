package app

import (
	"fmt"
	"strings"
	"time"

	"tableflip.dev/bujo/pkg/collection"
	cachepkg "tableflip.dev/bujo/pkg/tui/cache"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/events"
)

func findBulletInSnapshot(collections []collectiondetail.Section, collectionID, bulletID string) *collectiondetail.Bullet {
	collectionID = strings.TrimSpace(collectionID)
	bulletID = strings.TrimSpace(bulletID)
	if collectionID == "" || bulletID == "" {
		return nil
	}
	for idx := range collections {
		section := &collections[idx]
		if section == nil || strings.TrimSpace(section.ID) != collectionID {
			continue
		}
		for i := range section.Bullets {
			bullet := &section.Bullets[i]
			if strings.TrimSpace(bullet.ID) == bulletID {
				return bullet
			}
		}
	}
	return nil
}

func todayLabels(now time.Time) (string, string, string) {
	if now.IsZero() {
		now = time.Now()
	}
	month := now.Format("January 2006")
	day := now.Format("January 2, 2006")
	resolved := fmt.Sprintf("%s/%s", month, day)
	return month, day, resolved
}

func todayCollectionRefFromSections(sections []collectiondetail.Section, now time.Time) (events.CollectionRef, bool) {
	month, day, resolved := todayLabels(now)
	monthTime, _ := time.Parse("January 2006", month)
	dayTime, _ := time.Parse("January 2, 2006", day)
	ref := events.CollectionRef{
		ID:       resolved,
		Name:     day,
		ParentID: month,
		Type:     collection.TypeDaily,
		Month:    monthTime,
		Day:      dayTime,
	}
	for _, sec := range sections {
		if strings.EqualFold(strings.TrimSpace(sec.ID), resolved) {
			return ref, true
		}
	}
	return ref, false
}

func todayCollectionRefFromCache(cache *cachepkg.Cache, now time.Time) (events.CollectionRef, bool) {
	if cache == nil {
		return events.CollectionRef{}, false
	}
	month, day, resolved := todayLabels(now)
	monthTime, _ := time.Parse("January 2006", month)
	dayTime, _ := time.Parse("January 2, 2006", day)
	ref := events.CollectionRef{
		ID:       resolved,
		Name:     day,
		ParentID: month,
		Type:     collection.TypeDaily,
		Month:    monthTime,
		Day:      dayTime,
	}
	if _, ok := cache.SectionSnapshot(resolved); ok {
		return ref, true
	}
	return ref, false
}
