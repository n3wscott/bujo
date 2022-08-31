package ui

import (
	"tableflip.dev/bujo/pkg/entry"
	"tableflip.dev/bujo/pkg/glyph"
)

func StaticDemo() []*entry.Entry {
	e := make([]*entry.Entry, 0, 5)

	e = append(e,
		entry.New("Example", glyph.Note, "this is a note"),
		entry.New("Example", glyph.Note, "this is another note"),
		entry.New("Example", glyph.Task, "do this task"),
		entry.New("Example", glyph.Task, "this task is better..."),
		entry.New("Example", glyph.Event, "there is an event too"),
		entry.New("Example", glyph.Completed, "wish I had more things done"),
		entry.New("Example", glyph.Irrelevant, "not gonna do this"),
		entry.New("Today", glyph.Task, "task 1"),
		entry.New("Today", glyph.Task, "task 2"),
		entry.New("Today", glyph.MovedFuture, "task 3 went to future"),
		entry.New("Today", glyph.Task, "task 4"),
		entry.New("Future", glyph.Task, "t asddasd asasd a sda sddas  1"),
		entry.New("Future", glyph.MovedCollection, "tdasdasask 2 moved to a collection"),
		entry.New("Future", glyph.Task, "tdadsd asihd asiu hgadsui ask 3"),
		entry.New("Future", glyph.Task, "iadsbnliuasdb iudbhas jadshiads hdas luihadsui 4"),
	)

	e[2].Signifier = glyph.Priority
	e[3].Signifier = glyph.Priority
	e[5].Signifier = glyph.Investigation
	e[8].Signifier = glyph.Inspiration
	e[11].Signifier = glyph.Priority
	e[12].Signifier = glyph.Inspiration

	return e
}
