// Package complete provides the runner logic for marking entries complete.
package complete

import (
	"context"
	"errors"
	"fmt"
	"tableflip.dev/bujo/pkg/printers"
	"tableflip.dev/bujo/pkg/store"
)

// Complete marks an entry as completed.
type Complete struct {
	ID          string
	Persistence store.Persistence
}

// Do executes the completion operation for the configured entry ID.
func (n *Complete) Do(ctx context.Context) error {
	pp := printers.PrettyPrint{ShowID: true}

	if n.Persistence == nil {
		return errors.New("can not complete, no persistence")
	}

	collection := ""
	all := n.Persistence.ListAll(ctx)
	for _, e := range all {
		if e.ID == n.ID {
			e.Complete()
			if err := n.Persistence.Store(e); err != nil {
				return err
			}
			collection = e.Collection
			break
		}
	}

	all = n.Persistence.List(ctx, collection)
	fmt.Println("")
	pp.Title(collection)
	pp.Collection(all...)

	return nil
}
