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

	"tableflip.dev/bujo/pkg/entry"
)

// Persistence defines the persistence contract for journal entries.
type Persistence interface {
	MapAll(ctx context.Context) map[string][]*entry.Entry
	ListAll(ctx context.Context) []*entry.Entry
	List(ctx context.Context, collection string) []*entry.Entry
	Collections(ctx context.Context, prefix string) []string
	Store(e *entry.Entry) error
	Delete(e *entry.Entry) error
	EnsureCollection(collection string) error
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
	if err := json.Unmarshal(val, &e); err != nil {
		return nil, err
	}
	if e.Schema == "" {
		e.Schema = entry.CurrentSchema
	}
	pk := keyToPathTransform(key)
	e.ID = pk.FileName
	e.EnsureHistorySeed()
	return &e, nil
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
	all := make(map[string]string, 0)
	for key := range p.d.Keys(ctx.Done()) {
		pk := keyToPathTransform(key)
		ck := fromCollection(pk.Path[0])

		if strings.HasPrefix(ck, prefix) {
			if _, ok := all[ck]; !ok {
				all[ck] = ck
			}
		}
	}

	if idx, err := p.loadCollectionsIndex(); err == nil {
		for name := range idx {
			if strings.HasPrefix(name, prefix) {
				all[name] = name
			}
		}
	} else {
		fmt.Fprintf(os.Stderr, "store: load collections index: %v\n", err)
	}

	keys := make([]string, len(all))
	i := 0
	for k := range all {
		keys[i] = k
		i++
	}
	return keys
}

func (p *persistence) EnsureCollection(collection string) error {
	name := strings.TrimSpace(collection)
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
	if _, exists := index[name]; exists {
		return nil
	}
	index[name] = struct{}{}
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

func (p *persistence) loadCollectionsIndex() (map[string]struct{}, error) {
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
			return make(map[string]struct{}), nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return make(map[string]struct{}), nil
	}
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	index := make(map[string]struct{}, len(list))
	for _, name := range list {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		index[name] = struct{}{}
	}
	return index, nil
}

func (p *persistence) saveCollectionsIndex(idx map[string]struct{}) error {
	if p.basePath == "" {
		return errors.New("store: base path unknown")
	}
	if err := os.MkdirAll(p.basePath, 0o755); err != nil {
		return err
	}
	list := make([]string, 0, len(idx))
	for name := range idx {
		list = append(list, name)
	}
	sort.Strings(list)
	data, err := json.MarshalIndent(list, "", "  ")
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
