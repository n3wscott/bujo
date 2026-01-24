package cache

import (
	"context"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
)

// Service describes the journal data access needed by the cache.
type Service interface {
	CollectionsMeta(ctx context.Context, prefix string) ([]collection.Meta, error)
	Entries(ctx context.Context, collectionID string) ([]*entry.Entry, error)
	Add(ctx context.Context, collection string, bullet glyph.Bullet, msg string, sig glyph.Signifier) (*entry.Entry, error)
	SetParent(ctx context.Context, id, parentID string) (*entry.Entry, error)
}
