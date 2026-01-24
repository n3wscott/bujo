package addtask

import (
	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/tui/components/collectiondetail"
)

// Cache describes the minimal cache API needed by the add-task overlay.
type Cache interface {
	CollectionsMeta() []collection.Meta
	SectionSnapshot(id string) (collectiondetail.Section, bool)
	CreateBulletWithMeta(collectionID string, bullet collectiondetail.Bullet, meta map[string]string) error
}
