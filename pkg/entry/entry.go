package entry

import (
	"fmt"
	"github.com/n3wscott/bujo/pkg/glyph"
	"time"
)

const (
	CurrentSchema = "v0" // "v0" is also ""
)

func New(collection string, bullet glyph.Bullet, message string) *Entry {
	return &Entry{
		Schema:     CurrentSchema,
		Created:    Timestamp{Time: time.Now()},
		Collection: collection,
		Signifier:  glyph.None,
		Bullet:     bullet,
		Message:    message,
	}
}

type Entry struct {
	ID         string          `json:"-"` // do not json. ID is the filename.
	Schema     string          `json:"schema"`
	Created    Timestamp       `json:"created"`
	Collection string          `json:"collection"`
	Signifier  glyph.Signifier `json:"signifier,omitempty"`
	Bullet     glyph.Bullet    `json:"bullet,omitempty"`
	Message    string          `json:"message,omitempty"`
}

func (e *Entry) Complete() {
	e.Bullet = glyph.Completed
}

func (e *Entry) Strike() {
	e.Bullet = glyph.Irrelevant
	e.Signifier = glyph.None
}

func (e *Entry) Move(bullet glyph.Bullet, collection string) *Entry {
	ne := &Entry{
		ID:         "", // generate new id.
		Schema:     CurrentSchema,
		Created:    e.Created,
		Collection: collection,
		Signifier:  e.Signifier,
		Bullet:     e.Bullet,
		Message:    e.Message,
	}
	e.Bullet = bullet
	return ne
}

func (e *Entry) Title() string {
	return e.Collection
}

func (e *Entry) String() string {
	switch e.Bullet {
	case glyph.Completed:
		return fmt.Sprintf("%s %s  %s", glyph.None.String(), e.Bullet.String(), e.Message)
	default:
		return fmt.Sprintf("%s %s  %s", e.Signifier.String(), e.Bullet.String(), e.Message)
	}
}
