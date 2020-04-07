package track

import (
	"context"
	"errors"
	"fmt"
	"github.com/n3wscott/bujo/pkg/entry"
	"github.com/n3wscott/bujo/pkg/glyph"
	"github.com/n3wscott/bujo/pkg/printers"
	"github.com/n3wscott/bujo/pkg/store"
)

type Track struct {
	Collection  string
	Persistence store.Persistence
}

func (n *Track) Do(ctx context.Context) error {

	pp := printers.PrettyPrint{}

	if n.Persistence == nil {
		return errors.New("can not get, no persistence")
	}
	fmt.Println("")

	e := entry.New(n.Collection, glyph.Occurrence, "")
	if err := n.Persistence.Store(e); err != nil {
		return err
	}

	all := n.Persistence.List(ctx, n.Collection)
	pp.Title(n.Collection)
	pp.Tracking(all...)

	return nil
}
