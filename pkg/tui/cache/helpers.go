package cache

import (
	"strings"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/collection/viewmodel"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
	"tableflip.dev/bujo/pkg/tui/uiutil"
)

// cloneParsed deep-copies a parsed collection tree for safe read-only snapshots.
func cloneParsed(list []*viewmodel.ParsedCollection) []*viewmodel.ParsedCollection {
	if len(list) == 0 {
		return nil
	}
	out := make([]*viewmodel.ParsedCollection, 0, len(list))
	for _, item := range list {
		if item == nil {
			out = append(out, nil)
			continue
		}
		cloned := *item
		if len(item.Days) > 0 {
			cloned.Days = append([]viewmodel.DaySummary(nil), item.Days...)
		}
		cloned.Children = cloneParsed(item.Children)
		out = append(out, &cloned)
	}
	return out
}

// cloneSections deep-copies detail sections, including bullets.
func cloneSections(list []collectiondetail.Section) []collectiondetail.Section {
	if len(list) == 0 {
		return nil
	}
	out := make([]collectiondetail.Section, len(list))
	for i, sec := range list {
		out[i] = sec
		out[i].Bullets = cloneBullets(sec.Bullets)
	}
	return out
}

func cloneMetas(list []collection.Meta) []collection.Meta {
	if len(list) == 0 {
		return nil
	}
	out := make([]collection.Meta, len(list))
	copy(out, list)
	return out
}

func cloneBullets(list []collectiondetail.Bullet) []collectiondetail.Bullet {
	if len(list) == 0 {
		return nil
	}
	out := make([]collectiondetail.Bullet, len(list))
	for i, bullet := range list {
		out[i] = bullet
		out[i].Children = cloneBullets(bullet.Children)
	}
	return out
}

func leafName(path string) string {
	return uiutil.LastSegment(path)
}

func updateBulletByID(list *[]collectiondetail.Bullet, bullet collectiondetail.Bullet) bool {
	if list == nil || bullet.ID == "" {
		return false
	}
	items := *list
	for i := range items {
		if items[i].ID == bullet.ID {
			items[i] = mergeDetailBullet(items[i], bullet)
			*list = items
			return true
		}
		if len(items[i].Children) > 0 {
			if updateBulletByID(&items[i].Children, bullet) {
				*list = items
				return true
			}
		}
	}
	return false
}

// mergeDetailBullet overlays non-empty fields from updated onto base, preserving children.
func mergeDetailBullet(base, updated collectiondetail.Bullet) collectiondetail.Bullet {
	merged := base
	if strings.TrimSpace(updated.ID) != "" {
		merged.ID = updated.ID
	}
	if updated.Label != "" {
		merged.Label = updated.Label
	}
	if updated.Note != "" {
		merged.Note = updated.Note
	}
	if updated.Bullet != "" {
		merged.Bullet = updated.Bullet
	}
	if updated.Signifier != "" {
		merged.Signifier = updated.Signifier
	}
	if !updated.Created.IsZero() {
		merged.Created = updated.Created
	}
	merged.Locked = updated.Locked
	if len(updated.Children) > 0 {
		if len(merged.Children) == 0 {
			merged.Children = cloneBullets(updated.Children)
		} else {
			merged.Children = dedupeBullets(append(cloneBullets(merged.Children), updated.Children...))
		}
	}
	return merged
}
