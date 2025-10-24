package store

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

func (p *persistence) read(key string) (*entry.Entry, error) {
	val, err := p.d.Read(key)
	if err != nil {
		return nil, err
	}
	e := entry.Entry{}
	target := &e
	if err := json.Unmarshal(val, target); err != nil {
		var list []*entry.Entry
		if err2 := json.Unmarshal(val, &list); err2 == nil && len(list) > 0 && list[0] != nil {
			target = list[0]
		} else {
			return nil, err
		}
	}
	if target.Schema == "" {
		target.Schema = entry.CurrentSchema
	}
	pk := keyToPathTransform(key)
	target.ID = pk.FileName
	target.EnsureHistorySeed()
	return target, nil
}

func (p *persistence) MapAll(ctx context.Context) map[string][]*entry.Entry {
	all := make(map[string][]*entry.Entry, 0)
	for key := range p.d.Keys(ctx.Done()) {
		pk := keyToPathTransform(key)
		ck := fromCollection(pk.Path[0])

		e, err := p.read(key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", key, err)
			continue
		}

		if c, ok := all[ck]; !ok {
			all[ck] = []*entry.Entry{e}
		} else {
			all[ck] = append(c, e)
		}
	}
	for key := range all {
		sortEntries(all[key])
	}
	return all
}

func (p *persistence) ListAll(ctx context.Context) []*entry.Entry {
	all := make([]*entry.Entry, 0)
	for key := range p.d.Keys(ctx.Done()) {
		e, err := p.read(key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", key, err)
			continue
		}
		all = append(all, e)
	}
	sortEntries(all)
	return all
}

func (p *persistence) List(ctx context.Context, collection string) []*entry.Entry {
	ck := toCollection(collection)
	all := make([]*entry.Entry, 0)
	for key := range p.d.Keys(ctx.Done()) {
		if pk := keyToPathTransform(key); pk.Path[0] == ck {
			e, err := p.read(key)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s\n", key, err)
				continue
			}
			all = append(all, e)
		}
	}
	sortEntries(all)
	return all
}

func (p *persistence) Store(e *entry.Entry) error {
	if e.Schema == "" {
		e.Schema = entry.CurrentSchema
	}
	e.EnsureHistorySeed()
	key := toKey(e)
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if err := p.d.Write(key, data); err != nil {
		return err
	}
	return nil
}

func (p *persistence) Delete(e *entry.Entry) error {
	if e.Schema == "" {
		e.Schema = entry.CurrentSchema
	}
	key := toKey(e)
	return p.d.Erase(key)
}

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
		pk := keyToPathTransform(key)
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

const (
	layoutISO            = "2006-01-02"
	collectionsIndexFile = ".collections.json"
)

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

func sortEntries(entries []*entry.Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		left := entries[i]
		right := entries[j]
		if left == nil || right == nil {
			return left != nil
		}
		lt := left.Created.Time
		rt := right.Created.Time
		switch {
		case lt.IsZero() && rt.IsZero():
			return left.ID < right.ID
		case lt.IsZero():
			return false
		case rt.IsZero():
			return true
		default:
			if lt.Equal(rt) {
				return left.ID < right.ID
			}
			return lt.Before(rt)
		}
	})
}

func keyToPathTransform(s string) *diskv.PathKey {
	parts := strings.Split(s, "-")
	return &diskv.PathKey{
		Path:     parts[:len(parts)-1],
		FileName: parts[len(parts)-1],
	}
}

func pathToKeyTransform(pathKey *diskv.PathKey) string {
	return fmt.Sprintf("%s-%s", strings.Join(pathKey.Path, "-"), pathKey.FileName)
}

// toKey makes `collection-date-id`
func toKey(e *entry.Entry) string {
	collection := toCollection(e.Collection)
	then := e.Created.Format(layoutISO)

	if e.ID == "" {
		b, _ := json.Marshal(e)
		id := md5.Sum(b)
		e.ID = fmt.Sprintf("%x", id[:8])
	}

	return fmt.Sprintf("%s-%s-%s", collection, then, e.ID)
}

func toCollection(s string) string {
	collection := base64.StdEncoding.EncodeToString([]byte(s))
	return collection
}

func fromCollection(s string) string {
	collection, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return fmt.Sprintf("fromCollection: %s", err)
	}
	return string(collection)
}
