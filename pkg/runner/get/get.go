package get

import (
	"context"
	"errors"
	"fmt"
	"github.com/n3wscott/bujo/pkg/entry"
	"github.com/n3wscott/bujo/pkg/glyph"
	"github.com/n3wscott/bujo/pkg/printers"
	"github.com/n3wscott/bujo/pkg/store"
	"time"
)

type Get struct {
	ShowID      bool
	Bullet      glyph.Bullet
	Collection  string
	Persistence store.Persistence
}

// TODO: make the today logic a base thing or something.
const (
	layoutISO = "2006-01-02"
	layoutUS  = "January 2, 2006"
)

func (n *Get) Do(ctx context.Context) error {
	if n.Collection == "today" {
		n.Collection = time.Now().Format(layoutUS)
	}
	if n.Persistence == nil {
		return errors.New("can not get, no persistence")
	}

	switch n.Bullet {
	case glyph.Occurrence:
		return n.asTrack(ctx)
	default:
		return n.asCollection(ctx)
	}
}

func (n *Get) asTrack(ctx context.Context) error {
	if n.Collection == "" {
		return errors.New("a collection is required for trackers")
	}

	pp := printers.PrettyPrint{} // show id not supported for tracks yet.

	fmt.Println("")

	all := n.Persistence.List(ctx, n.Collection)

	pp.Title(n.Collection)
	pp.Tracking(all...)

	return nil
}

func (n *Get) asCollection(ctx context.Context) error {
	pp := printers.PrettyPrint{ShowID: n.ShowID}

	fmt.Println("")

	if n.Collection != "" {
		all := n.Persistence.List(ctx, n.Collection)
		all = n.filtered(all)

		pp.Title(n.Collection)
		pp.Collection(all...)

		return nil
	}

	allm := n.Persistence.MapAll(ctx)
	for c, all := range allm {
		all = n.filtered(all)
		if len(all) == 0 {
			continue
		}
		pp.Title(c)
		pp.Collection(all...)
	}

	return nil
}

func (n *Get) filtered(all []*entry.Entry) []*entry.Entry {
	c := make([]*entry.Entry, 0, len(all))
	for _, a := range all {
		if n.Bullet == glyph.Any || n.Bullet == a.Bullet {
			c = append(c, a)
		}
	}
	return c
}
