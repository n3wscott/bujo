package add

import (
	"context"
	"github.com/n3wscott/bujo/pkg/printers"
	"github.com/n3wscott/bujo/pkg/store"
	"time"

	"github.com/n3wscott/bujo/pkg/entry"
	"github.com/n3wscott/bujo/pkg/glyph"
)

type Add struct {
	Entry entry.Entry

	Bullet        glyph.Bullet
	Collection    string
	Message       string
	On            *time.Time
	Priority      bool
	Inspiration   bool
	Investigation bool

	Persistence store.Persistence
}

const (
	layoutISO = "2006-01-02"
	layoutUS  = "January 2, 2006"
)

func (n *Add) Do(ctx context.Context) error {
	if n.Collection == "today" {
		n.Collection = time.Now().Format(layoutUS)
	}

	e := entry.New(n.Collection, n.Bullet, n.Message)

	if n.On != nil {
		e.On = &entry.Timestamp{Time: *n.On}
	}

	switch {
	case n.Priority:
		e.Signifier = glyph.Priority
	case n.Inspiration:
		e.Signifier = glyph.Inspiration
	case n.Investigation:
		e.Signifier = glyph.Investigation
	}

	pp := printers.PrettyPrint{}
	pp.Title(e.Collection)
	if n.Persistence != nil {
		if err := n.Persistence.Store(e); err != nil {
			return err
		}
		all := n.Persistence.List(ctx, e.Collection)
		pp.Collection(all...)
	} else {
		pp.Collection(e)
	}

	return nil
}
