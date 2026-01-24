package store

import (
	"context"

	"github.com/peterbourgon/diskv/v3"

	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/entry"
)

// Persistence defines the persistence contract for journal entries.
type Persistence interface {
	MapAll(ctx context.Context) map[string][]*entry.Entry
	ListAll(ctx context.Context) []*entry.Entry
	List(ctx context.Context, collection string) []*entry.Entry
	Collections(ctx context.Context, prefix string) []string
	CollectionsMeta(ctx context.Context, prefix string) []collection.Meta
	Store(e *entry.Entry) error
	Delete(e *entry.Entry) error
	DeleteCollection(ctx context.Context, collection string) error
	EnsureCollection(collection string) error
	EnsureCollectionTyped(collection string, typ collection.Type) error
	SetCollectionType(collection string, typ collection.Type) error
	Watch(ctx context.Context) (<-chan Event, error)
}

// Load creates a Persistence backed by diskv using the provided config.
func Load(cfg Config) (Persistence, error) {
	if cfg == nil {
		var err error
		cfg, err = LoadConfig()
		if err != nil {
			return nil, err
		}
	}

	basePath := cfg.BasePath()
	return &persistence{d: diskv.New(diskv.Options{
		BasePath:          basePath,
		AdvancedTransform: keyToPathTransform,
		InverseTransform:  pathToKeyTransform,
		CacheSizeMax:      1024 * 1024, // 1MB
	}), basePath: basePath}, nil
}

type persistence struct {
	d        *diskv.Diskv
	basePath string
}
