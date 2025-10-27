package main

import (
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/events"
)

const testbedFeedComponent = events.ComponentID("TestbedFeed")

type heldBullet struct {
	collection events.CollectionViewRef
	bullet     events.BulletRef
}

type bulletFeeder struct {
	queue []heldBullet
}

func newBulletFeeder(items []heldBullet) bulletFeeder {
	return bulletFeeder{queue: append([]heldBullet(nil), items...)}
}

func (f *bulletFeeder) NextCmd(component events.ComponentID) tea.Cmd {
	if len(f.queue) == 0 {
		return nil
	}
	next := f.queue[0]
	f.queue = f.queue[1:]
	return events.BulletChangeCmd(component, events.ChangeCreate, next.collection, next.bullet, nil)
}

func applyHoldback(sections []collectiondetail.Section, hold int) ([]collectiondetail.Section, []heldBullet, error) {
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
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})
	selected := candidates[:hold]
	releaseOrder := make([]bulletLocation, len(selected))
	copy(releaseOrder, selected)
	removalOrder := make([]bulletLocation, len(selected))
	copy(removalOrder, selected)
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
	for _, loc := range releaseOrder {
		bullet, ok := removed[locKey(loc)]
		if !ok {
			continue
		}
		section := sections[loc.sectionIdx]
		held = append(held, heldBullet{
			collection: events.CollectionViewRef{
				ID:       section.ID,
				Title:    section.Title,
				Subtitle: section.Subtitle,
			},
			bullet: bulletRefFromDetail(bullet),
		})
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

func bulletRefFromDetail(b collectiondetail.Bullet) events.BulletRef {
	return events.BulletRef{
		ID:        b.ID,
		Label:     b.Label,
		Note:      b.Note,
		Bullet:    b.Bullet,
		Signifier: b.Signifier,
	}
}
