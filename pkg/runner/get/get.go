package get

import (
	"context"
	"errors"
	"fmt"
	"github.com/n3wscott/bujo/pkg/entry"
	"github.com/n3wscott/bujo/pkg/glyph"
	"github.com/n3wscott/bujo/pkg/store"
	"time"
)

type Get struct {
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
	fmt.Println("")

	if n.Collection != "" {
		all := n.Persistence.List(ctx, n.Collection)
		n.printFiltered(all)
		return nil
	}

	allm := n.Persistence.ListAll(ctx)
	for _, all := range allm {
		n.printFiltered(all)
	}

	return nil
}

func (n *Get) printFiltered(all []*entry.Entry) {
	c := make([]*entry.Entry, 0, len(all))
	for _, a := range all {
		if n.Bullet == glyph.Any || n.Bullet == a.Bullet {
			c = append(c, a)
		}
	}
	entry.PrettyPrintCollection(c...)
	fmt.Println("")
}
