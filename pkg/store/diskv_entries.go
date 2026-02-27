package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"tableflip.dev/bujo/pkg/entry"
)

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
	target.NormalizeLabels()
	target.EnsureHistorySeed()
	return target, nil
}

func (p *persistence) MapAll(ctx context.Context) map[string][]*entry.Entry {
	all := make(map[string][]*entry.Entry, 0)
	for key := range p.d.Keys(ctx.Done()) {
		if key == collectionsIndexFile {
			continue
		}
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
		if key == collectionsIndexFile {
			continue
		}
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
		if key == collectionsIndexFile {
			continue
		}
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
	e.NormalizeLabels()
	e.EnsureHistorySeed()
	key := toKey(e)
	if err := p.removeStaleCopies(e, key); err != nil {
		return err
	}
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
	if e == nil {
		return nil
	}
	if e.Schema == "" {
		e.Schema = entry.CurrentSchema
	}
	key := toKey(e)
	if err := p.d.Erase(key); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if strings.TrimSpace(e.ID) == "" {
		return nil
	}

	ctx := context.Background()
	deleted := false
	for existingKey := range p.d.Keys(ctx.Done()) {
		if existingKey == collectionsIndexFile {
			continue
		}
		pk := keyToPathTransform(existingKey)
		if pk.FileName != e.ID {
			continue
		}
		if err := p.d.Erase(existingKey); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		deleted = true
	}
	if deleted {
		return nil
	}
	return os.ErrNotExist
}

func (p *persistence) removeStaleCopies(e *entry.Entry, currentKey string) error {
	if e == nil || strings.TrimSpace(e.ID) == "" {
		return nil
	}
	ctx := context.Background()
	for key := range p.d.Keys(ctx.Done()) {
		if key == collectionsIndexFile || key == currentKey {
			continue
		}
		pk := keyToPathTransform(key)
		if pk.FileName == e.ID {
			if err := p.d.Erase(key); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
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
