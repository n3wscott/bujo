package strike

import (
	"context"
	"errors"
	"fmt"
	"github.com/n3wscott/bujo/pkg/printers"
	"github.com/n3wscott/bujo/pkg/store"
)

type Strike struct {
	ID          string
	Persistence store.Persistence
}

func (n *Strike) Do(ctx context.Context) error {
	pp := printers.PrettyPrint{ShowID: true}

	if n.Persistence == nil {
		return errors.New("can not strike, no persistence")
	}

	collection := ""
	all := n.Persistence.ListAll(ctx)
	for _, e := range all {
		if e.ID == n.ID {
			e.Strike()
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
