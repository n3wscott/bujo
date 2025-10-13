// Package track provides runners that summarize entry tracking data.
package track

import (
	"context"
	"errors"
	"fmt"
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/printers"
	"tableflip.dev/bujo/pkg/store"
)

// Track records an occurrence entry for a collection.
type Track struct {
	Collection  string
	Persistence store.Persistence
}

// Do writes an occurrence entry and reprints the tracking summary.
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
