package store

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/peterbourgon/diskv/v3"

	"tableflip.dev/bujo/pkg/entry"
)

const (
	layoutISO            = "2006-01-02"
	collectionsIndexFile = ".collections.json"
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

// toKey makes `collection-date-id`.
func toKey(e *entry.Entry) string {
	collection := toCollection(e.Collection)
	then := e.Created.UTC().Format(layoutISO)

	if e.ID == "" {
		b, _ := json.Marshal(e)
		id := md5.Sum(b)
		e.ID = fmt.Sprintf("%x", id[:8])
	}

	return fmt.Sprintf("%s-%s-%s", collection, then, e.ID)
}

func toCollection(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func fromCollection(s string) string {
	collection, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return fmt.Sprintf("fromCollection: %s", err)
	}
	return string(collection)
}
