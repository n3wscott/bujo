package main

import (
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"tableflip.dev/bujo/pkg/collection"
	cachepkg "tableflip.dev/bujo/pkg/tui/cache"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/events"
)

const testbedFeedComponent = events.ComponentID("TestbedFeed")

type heldBullet struct {
	section         collectiondetail.Section
	meta            collection.Meta
	bullet          collectiondetail.Bullet
	needsCollection bool
}

type bulletFeeder struct {
	cache *cachepkg.Cache
	queue []heldBullet
}

func newBulletFeeder(cache *cachepkg.Cache, items []heldBullet) bulletFeeder {
	return bulletFeeder{
		cache: cache,
		queue: append([]heldBullet(nil), items...),
	}
}

func (f *bulletFeeder) Next() {
	if len(f.queue) == 0 || f.cache == nil {
		return
	}
	next := f.queue[0]
	f.queue = f.queue[1:]
	if next.needsCollection {
		f.cache.CreateCollection(next.meta)
	}
	f.cache.CreateBullet(next.meta.Name, next.bullet)
}

func applyHoldback(sections []collectiondetail.Section, hold int, metaIndex map[string]collection.Meta) ([]collectiondetail.Section, []heldBullet, error) {
	if hold <= 0 {
		return sections, nil, nil
	}
	candidates := collectBulletCandidates(sections)
	if len(candidates) == 0 {
		return sections, nil, nil
	}
	if hold > len(candidates) {
		hold = len(candidates)
	}
	totalPerSection := make(map[int]int, len(sections))
	for _, loc := range candidates {
		totalPerSection[loc.sectionIdx]++
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})
	selected := candidates[:hold]
	releaseOrder := make([]bulletLocation, len(selected))
	copy(releaseOrder, selected)
	removalOrder := make([]bulletLocation, len(selected))
	copy(removalOrder, selected)
	heldPerSection := make(map[int]int, len(selected))
	for _, loc := range selected {
		heldPerSection[loc.sectionIdx]++
	}
	sort.SliceStable(removalOrder, func(i, j int) bool {
		a := removalOrder[i]
		b := removalOrder[j]
		if a.sectionIdx != b.sectionIdx {
			return a.sectionIdx > b.sectionIdx
		}
		if len(a.path) != len(b.path) {
			return len(a.path) > len(b.path)
		}
		for k := 0; k < len(a.path) && k < len(b.path); k++ {
			if a.path[k] != b.path[k] {
				return a.path[k] > b.path[k]
			}
		}
		return false
	})

	removed := make(map[string]collectiondetail.Bullet, len(removalOrder))
	for _, loc := range removalOrder {
		if bullet, ok := removeBulletAt(sections, loc); ok {
			removed[locKey(loc)] = bullet
		}
	}

	held := make([]heldBullet, 0, len(selected))
	releasedPerSection := make(map[int]int, len(selected))
	for _, loc := range releaseOrder {
		bullet, ok := removed[locKey(loc)]
		if !ok {
			continue
		}
		section := sections[loc.sectionIdx]
		meta := metaIndex[strings.TrimSpace(section.ID)]
		if meta.Name == "" {
			meta.Name = strings.TrimSpace(section.ID)
		}
		if meta.Type == "" {
			meta.Type = collection.TypeGeneric
		}
		totalLeaf := totalPerSection[loc.sectionIdx]
		heldLeaf := heldPerSection[loc.sectionIdx]
		released := releasedPerSection[loc.sectionIdx]
		needsCollection := totalLeaf > 0 && heldLeaf == totalLeaf && released == 0
		releasedPerSection[loc.sectionIdx] = released + 1
		sectionTemplate := collectiondetail.Section{
			ID:       section.ID,
			Title:    section.Title,
			Subtitle: section.Subtitle,
		}
		sectionTemplate.Bullets = nil
		held = append(held, heldBullet{
			section:         sectionTemplate,
			meta:            meta,
			bullet:          bullet,
			needsCollection: needsCollection,
		})
	}
	if len(heldPerSection) > 0 {
		filtered := make([]collectiondetail.Section, 0, len(sections))
		for idx, sec := range sections {
			if totalPerSection[idx] > 0 && heldPerSection[idx] == totalPerSection[idx] {
				continue
			}
			filtered = append(filtered, sec)
		}
		sections = filtered
	}
	return sections, held, nil
}

type bulletLocation struct {
	sectionIdx int
	path       []int
}

func collectBulletCandidates(sections []collectiondetail.Section) []bulletLocation {
	var locs []bulletLocation
	for si, sec := range sections {
		var walk func(path []int, bullets []collectiondetail.Bullet)
		walk = func(path []int, bullets []collectiondetail.Bullet) {
			for idx, bullet := range bullets {
				nextPath := appendPath(path, idx)
				if len(bullet.Children) == 0 {
					locs = append(locs, bulletLocation{
						sectionIdx: si,
						path:       nextPath,
					})
				} else {
					walk(nextPath, bullet.Children)
				}
			}
		}
		walk(nil, sec.Bullets)
	}
	return locs
}

func appendPath(path []int, idx int) []int {
	next := make([]int, len(path)+1)
	copy(next, path)
	next[len(path)] = idx
	return next
}

func removeBulletAt(sections []collectiondetail.Section, loc bulletLocation) (collectiondetail.Bullet, bool) {
	if loc.sectionIdx < 0 || loc.sectionIdx >= len(sections) || len(loc.path) == 0 {
		return collectiondetail.Bullet{}, false
	}
	slice := &sections[loc.sectionIdx].Bullets
	for depth := 0; depth < len(loc.path); depth++ {
		idx := loc.path[depth]
		if idx < 0 || idx >= len(*slice) {
			return collectiondetail.Bullet{}, false
		}
		if depth == len(loc.path)-1 {
			removed := (*slice)[idx]
			*slice = append((*slice)[:idx], (*slice)[idx+1:]...)
			return removed, true
		}
		slice = &(*slice)[idx].Children
	}
	return collectiondetail.Bullet{}, false
}

func locKey(loc bulletLocation) string {
	var b strings.Builder
	b.WriteString(fmtInt(loc.sectionIdx))
	b.WriteByte(':')
	for i, v := range loc.path {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(fmtInt(v))
	}
	return b.String()
}

func fmtInt(v int) string {
	return strconv.Itoa(v)
}

func registerHeldTemplates(cache *cachepkg.Cache, held []heldBullet) {
	if cache == nil || len(held) == 0 {
		return
	}
	seen := make(map[string]struct{})
	for _, item := range held {
		id := strings.TrimSpace(item.section.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		cache.RegisterSectionTemplate(item.section)
		seen[id] = struct{}{}
	}
}
