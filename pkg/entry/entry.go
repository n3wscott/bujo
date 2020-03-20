package entry

import (
	"fmt"
	"github.com/n3wscott/bujo/pkg/glyph"
)

func New(collection string, bullet glyph.Bullet, message string) *Entry {
	return &Entry{
		Collection: collection,
		Signifier:  glyph.None,
		Bullet:     bullet,
		Message:    message,
	}
}

type Entry struct {
	Collection string          `json:"collection"`
	Signifier  glyph.Signifier `json:"signifier,omitempty"`
	Bullet     glyph.Bullet    `json:"bullet,omitempty"`
	Message    string          `json:"message,omitempty"`
}

func (e *Entry) Title() string {
	return e.Collection
}

func (e *Entry) Row() (string, string, string) {
	return e.Signifier.String(), e.Bullet.String(), e.Message
}

func (e *Entry) String() string {
	switch e.Bullet {
	case glyph.Completed:
		return fmt.Sprintf("%s %s  %s", glyph.None.String(), e.Bullet.String(), e.Message)
	default:
		return fmt.Sprintf("%s %s  %s", e.Signifier.String(), e.Bullet.String(), e.Message)
	}
}
