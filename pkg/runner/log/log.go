package log

import (
	"context"
	"errors"
	"tableflip.dev/bujo/pkg/glyph"
	"tableflip.dev/bujo/pkg/runner/get"
	"tableflip.dev/bujo/pkg/store"
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

	// TODO: future log tasks can be scheduled on a date and are rendered with the day of the month after the bullet, like:
	// Future - April, 2020
	//  ⦁ 21: This event is happening WAY later maybe, if I get to it.
	//
	if n.Future {
		collection := n.On.Format(layoutUSFutureMonth)
		g := get.Get{
			Bullet:      glyph.Any, //  Really this should filter on tasks and events.
			Collection:  collection,
			Persistence: n.Persistence,
		}
		if err := g.Do(ctx); err != nil {
			return err
		}
	}

	// Calendar View.
	if n.Month {
		collection := n.On.Format(layoutUSMonth)
		g := get.Get{
			CalendarView: true,
			Bullet:       glyph.Event,
			Collection:   collection,
			Persistence:  n.Persistence,
			On:           n.On,
		}
		if err := g.Do(ctx); err != nil {
			return err
		}
	}

	// Task View.
	if n.Month {
		collection := n.On.Format(layoutUSMonth)
		g := get.Get{
			Bullet:      glyph.Task,
			Collection:  collection,
			Persistence: n.Persistence,
			On:          n.On,
		}
		if err := g.Do(ctx); err != nil {
			return err
		}
	}

	// Day view.
	if n.Day {
		collection := n.On.Format(layoutUSDay)
		g := get.Get{
			Bullet:      glyph.Any,
			Collection:  collection,
			Persistence: n.Persistence,
		}
		if err := g.Do(ctx); err != nil {
			return err
		}
	}

	return nil
}
