package entry

import "github.com/n3wscott/bujo/pkg/glyph"

func New(collection string, bullet glyph.Bullet, message string) *Entry {
	return &Entry{
		Collection: collection,
		Signifier:  glyph.None,
		Bullet:     bullet,
		Message:    message,
	}
}

type Entry struct {
	Collection string

	Signifier glyph.Signifier
	Bullet    glyph.Bullet
	Message   string
}

func (e *Entry) Title() string {
	return e.Collection
}

func (e *Entry) Row() (string, string, string) {
	return e.Signifier.String(), e.Bullet.String(), e.Message
}
