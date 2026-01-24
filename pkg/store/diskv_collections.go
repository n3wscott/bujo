package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"tableflip.dev/bujo/pkg/collection"
)

func (p *persistence) Collections(ctx context.Context, prefix string) []string {
	metas := p.CollectionsMeta(ctx, prefix)
	names := make([]string, len(metas))
	for i, meta := range metas {
		names[i] = meta.Name
	}
	return names
}

func (p *persistence) CollectionsMeta(ctx context.Context, prefix string) []collection.Meta {
	all := make(map[string]collection.Meta)
	if idx, err := p.loadCollectionsIndex(); err == nil {
		for name, meta := range idx {
			all[name] = meta
		}
	} else {
		fmt.Fprintf(os.Stderr, "store: load collections index: %v\n", err)
	}

	for key := range p.d.Keys(ctx.Done()) {
		if key == collectionsIndexFile {
			continue
		}
		pk := keyToPathTransform(key)
		if len(pk.Path) == 0 || pk.Path[0] == "" {
			continue
		}
		ck := fromCollection(pk.Path[0])

		meta, ok := all[ck]
		if !ok {
			meta = collection.Meta{Name: ck, Type: collection.TypeGeneric}
		}
		if meta.Name == "" {
			meta.Name = ck
		}
		if meta.Type == "" {
			meta.Type = collection.TypeGeneric
		}
		all[ck] = meta
	}

	list := make([]collection.Meta, 0, len(all))
	for name, meta := range all {
		if prefix == "" || strings.HasPrefix(name, prefix) {
			if meta.Name == "" {
				meta.Name = name
			}
			if meta.Type == "" {
				meta.Type = collection.TypeGeneric
			}
			list = append(list, meta)
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	return list
}

func (p *persistence) EnsureCollection(name string) error {
	return p.EnsureCollectionTyped(name, "")
}

func (p *persistence) EnsureCollectionTyped(name string, typ collection.Type) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("store: collection name required")
	}
	if p.basePath == "" {
		return errors.New("store: base path unknown")
	}
	if err := os.MkdirAll(p.basePath, 0o755); err != nil {
		return fmt.Errorf("store: ensure base path: %w", err)
	}
	encoded := toCollection(name)
	if err := os.MkdirAll(filepath.Join(p.basePath, encoded), 0o755); err != nil {
		return fmt.Errorf("store: ensure collection directory: %w", err)
	}
	index, err := p.loadCollectionsIndex()
	if err != nil {
		return fmt.Errorf("store: load collections index: %w", err)
	}
	meta := index[name]
	if meta.Name == "" {
		meta.Name = name
	}
	if typ != "" {
		meta.Type = typ
	}
	if meta.Type == "" {
		meta.Type = collection.TypeGeneric
	}
	index[name] = meta
	if err := p.saveCollectionsIndex(index); err != nil {
		return fmt.Errorf("store: save collections index: %w", err)
	}
	return nil
}

func (p *persistence) SetCollectionType(name string, typ collection.Type) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("store: collection name required")
	}
	index, err := p.loadCollectionsIndex()
	if err != nil {
		return fmt.Errorf("store: load collections index: %w", err)
	}
	meta := index[name]
	meta.Name = name
	meta.Type = typ
	index[name] = meta
	if err := p.saveCollectionsIndex(index); err != nil {
		return fmt.Errorf("store: save collections index: %w", err)
	}
	return nil
}

func (p *persistence) DeleteCollection(ctx context.Context, name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return errors.New("store: collection name required")
	}
	all := p.MapAll(ctx)
	prefix := trimmed + "/"
	removedCollections := make(map[string]struct{})
	for col, entries := range all {
		if col == trimmed || strings.HasPrefix(col, prefix) {
			for _, e := range entries {
				if err := p.Delete(e); err != nil {
					return err
				}
				removedCollections[col] = struct{}{}
			}
		}
	}
	index, err := p.loadCollectionsIndex()
	if err != nil {
		return fmt.Errorf("store: load collections index: %w", err)
	}
	for col := range index {
		if col == trimmed || strings.HasPrefix(col, prefix) {
			removedCollections[col] = struct{}{}
			delete(index, col)
		}
	}
	if len(removedCollections) == 0 {
		return fmt.Errorf("store: collection %q not found", trimmed)
	}
	if err := p.saveCollectionsIndex(index); err != nil {
		return fmt.Errorf("store: save collections index: %w", err)
	}
	for col := range removedCollections {
		encoded := toCollection(col)
		path := filepath.Join(p.basePath, encoded)
		_ = os.RemoveAll(path)
	}
	return nil
}

func (p *persistence) collectionsIndexPath() string {
	return filepath.Join(p.basePath, collectionsIndexFile)
}

func (p *persistence) loadCollectionsIndex() (map[string]collection.Meta, error) {
	if p.basePath == "" {
		return nil, errors.New("store: base path unknown")
	}
	if err := os.MkdirAll(p.basePath, 0o755); err != nil {
		return nil, err
	}
	path := p.collectionsIndexPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(map[string]collection.Meta), nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return make(map[string]collection.Meta), nil
	}
	list, err := collection.UnmarshalList(data)
	if err != nil {
		return nil, err
	}
	index := make(map[string]collection.Meta, len(list))
	for _, meta := range list {
		name := strings.TrimSpace(meta.Name)
		if name == "" {
			continue
		}
		if meta.Type == "" {
			meta.Type = collection.TypeGeneric
		}
		meta.Name = name
		index[name] = meta
	}
	return index, nil
}

func (p *persistence) saveCollectionsIndex(idx map[string]collection.Meta) error {
	if p.basePath == "" {
		return errors.New("store: base path unknown")
	}
	if err := os.MkdirAll(p.basePath, 0o755); err != nil {
		return err
	}
	list := make([]collection.Meta, 0, len(idx))
	for name, meta := range idx {
		if meta.Name == "" {
			meta.Name = name
		}
		if meta.Type == "" {
			meta.Type = collection.TypeGeneric
		}
		list = append(list, meta)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	data, err := collection.MarshalList(list)
	if err != nil {
		return err
	}
	path := p.collectionsIndexPath()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (p *persistence) CollectionsIndexExists() bool {
	if p == nil {
		return false
	}
	_, err := os.Stat(p.collectionsIndexPath())
	return err == nil
}
