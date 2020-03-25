package get

import (
	"context"
	"github.com/n3wscott/bujo/pkg/glyph"
	"github.com/n3wscott/bujo/pkg/store"
)

type Get struct {
	Bullet      glyph.Bullet
	Collection  string
	Persistence store.Persistence
}

func (n *Get) Do(ctx context.Context) error {
	//if n.Persistence != nil {
	//	if err := n.Persistence.Store(e); err != nil {
	//		return err
	//	}
	//	all := n.Persistence.List(ctx, e.Collection)
	//	entry.PrettyPrintCollection(all...)
	//} else {
	//	entry.PrettyPrintCollection(e)
	//}
	return nil
}
