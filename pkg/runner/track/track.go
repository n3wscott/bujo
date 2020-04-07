package track

import (
	"context"
	"errors"
	"fmt"
	"github.com/n3wscott/bujo/pkg/printers"
	"github.com/n3wscott/bujo/pkg/store"
)

type Track struct {
	Collection  string
	Persistence store.Persistence
}

// TODO: work in progress, trying to make a daily count like thing.

func (n *Track) Do(ctx context.Context) error {

	pp := printers.PrettyPrint{}

	if n.Persistence == nil {
		return errors.New("can not get, no persistence")
	}
	fmt.Println("")
	//
	//entry.New()
	//n.Persistence.Store(e)

	all := n.Persistence.List(ctx, n.Collection)
	//all = n.filtered(all)
	pp.Title(n.Collection)
	pp.Collection(all...)

	return nil
}
