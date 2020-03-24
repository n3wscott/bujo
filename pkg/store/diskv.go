package store

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/n3wscott/bujo/pkg/entry"
	"github.com/peterbourgon/diskv/v3"
	"strings"
)

type Persistence interface {
	ListAll() []*entry.Entry
	Store(e *entry.Entry) error
}

func Load(cfg Config) (Persistence, error) {
	if cfg == nil {
		var err error
		cfg, err = LoadConfig()
		if err != nil {
			return nil, err
		}
	}

	return &persistence{d: diskv.New(diskv.Options{
		BasePath:          cfg.BasePath(),
		AdvancedTransform: keyToPathTransform,
		InverseTransform:  pathToKeyTransform,
		CacheSizeMax:      1024 * 1024, // 1MB
	})}, nil
}

type persistence struct {
	d *diskv.Diskv
}

func (p *persistence) ListAll() []*entry.Entry {
	var keyCount int
	for key := range p.d.Keys(nil) {
		val, err := p.d.Read(key)
		if err != nil {
			panic(fmt.Sprintf("key %s had no value", key))
		}
		fmt.Printf("%s: %s\n", key, val)
		keyCount++
	}
	fmt.Printf("%d total keys\n", keyCount)
	return nil
}

func (p *persistence) Store(e *entry.Entry) error {
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

//func main() {
//
//	keys := []string{"a thing", "2016-02-21", "future"}
//
//	for i, valueStr := range []string{
//		"I am the very model of a modern Major-General",
//		"I've information vegetable, animal, and mineral",
//		"I know the kings of England, and I quote the fights historical",
//		"From Marathon to Waterloo, in order categorical",
//		"I'm very well acquainted, too, with matters mathematical",
//		"I understand equations, both the simple and quadratical",
//		"About binomial theorem I'm teeming with a lot o' news",
//		"With many cheerful facts about the square of the hypotenuse",
//	} {
//		key := toKey(keys[i%3], valueStr)
//		if err := d.Write(key, []byte(valueStr)); err != nil {
//			fmt.Printf("failed to write key, %s\n", err)
//		}
//	}
//
//	var keyCount int
//	for key := range d.Keys(nil) {
//		val, err := d.Read(key)
//		if err != nil {
//			panic(fmt.Sprintf("key %s had no value", key))
//		}
//		fmt.Printf("%s: %s\n", key, val)
//		keyCount++
//	}
//	fmt.Printf("%d total keys\n", keyCount)
//
//	// d.EraseAll() // leave it commented out to see how data is kept on disk
//}

const (
	layoutISO = "2006-01-02"
)

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
	collection := base64.StdEncoding.EncodeToString([]byte(e.Collection))
	then := e.Created.Time.Format(layoutISO)
	id := md5.Sum([]byte(e.Message))

	return fmt.Sprintf("%s-%s-%x", collection, then, id[:8])
}
