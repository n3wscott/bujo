package log

import (
	"context"
	"errors"
	"github.com/n3wscott/bujo/pkg/glyph"
	"github.com/n3wscott/bujo/pkg/runner/get"
	"github.com/n3wscott/bujo/pkg/store"
	"time"
)

type Log struct {
	Persistence store.Persistence
	Day         bool
	Month       bool
	Future      bool
	On          time.Time
	// TODO: a range.
}

// TODO: make the today logic a base thing or something.
const (
	layoutUSDay         = "January 2, 2006"
	layoutUSMonth       = "January, 2006"
	layoutUSFutureMonth = "Future - January, 2006"
)

func (n *Log) Do(ctx context.Context) error {

	//pp := printers.PrettyPrint{}

	if n.Persistence == nil {
		return errors.New("can not get, no persistence")
	}

	if n.Future {
		collection := n.On.Format(layoutUSFutureMonth)
		g := get.Get{
			ShowID:          false,
			ListCollections: false,
			Bullet:          glyph.Any, //  Really this should filter on tasks and events.
			Collection:      collection,
			Persistence:     n.Persistence,
		}
		if err := g.Do(ctx); err != nil {
			return err
		}
	}

	// This needs the calendar view.
	if n.Month {
		collection := n.On.Format(layoutUSMonth)
		g := get.Get{
			ShowID:          false,
			ListCollections: false,
			Bullet:          glyph.Task, // It should filter on tasks.
			Collection:      collection,
			Persistence:     n.Persistence,
		}
		if err := g.Do(ctx); err != nil {
			return err
		}
	}

	if n.Day {
		collection := n.On.Format(layoutUSDay)
		g := get.Get{
			ShowID:          false,
			ListCollections: false,
			Bullet:          glyph.Any,
			Collection:      collection,
			Persistence:     n.Persistence,
		}
		if err := g.Do(ctx); err != nil {
			return err
		}
	}

	return nil
}
