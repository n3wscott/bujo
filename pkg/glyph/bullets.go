package glyph

import (
	"fmt"
	"strings"
)

type Glyph struct {
	Key       string
	Symbol    string
	Meaning   string
	Aliases   []string
	Signifier bool
}

func DefaultGlyphs() []Glyph {
	g := make([]Glyph, 0, 9)

	g = append(g, Glyph{
		Key:     "+",
		Symbol:  "●",
		Meaning: "task",
		Aliases: []string{"+", "*", "task", "tasks"},
	}, Glyph{
		Key:     "x",
		Symbol:  "✘",
		Meaning: "task completed",
		Aliases: []string{"x", "completed", "completes", "complete", "done"},
	}, Glyph{
		Key:     ">",
		Symbol:  "›",
		Meaning: "task moved to collection",
		Aliases: []string{">", "move-collection", "moved-collection"},
	}, Glyph{
		Key:     "<",
		Symbol:  "‹",
		Meaning: "task moved to future log",
		Aliases: []string{"<", "move-future", "moved-future"},
	}, Glyph{
		Key:     "~",
		Symbol:  "⦵",
		Meaning: "task irrelevant",
		Aliases: []string{"~", "strike", "strikes", "striked"},
	}, Glyph{
		Key:     "-",
		Symbol:  "⁃",
		Meaning: "note",
		Aliases: []string{"-", "note", "notes", "noted"},
	}, Glyph{
		Key:     "o",
		Symbol:  "○",
		Meaning: "event",
		Aliases: []string{"o", "event", "events"},
	}, Glyph{
		Key:     "",
		Symbol:  "",
		Meaning: "any",
		Aliases: []string{"any"},
	}, Glyph{
		Key:       "*",
		Symbol:    "✷",
		Meaning:   "priority",
		Signifier: true,
	}, Glyph{
		Key:       "!",
		Symbol:    "!",
		Meaning:   "inspiration",
		Signifier: true,
	}, Glyph{
		Key:       "?",
		Symbol:    "?",
		Meaning:   "investigation",
		Signifier: true,
	}, Glyph{
		Key:       " ",
		Symbol:    " ",
		Meaning:   "none",
		Signifier: true,
	})

	return g
}

func (g Glyph) String() string {
	return g.Symbol
}

type Bullet int
type Signifier int

// the indexes of these `iota` enums line up with the indexes of DefaultGlyphs
const (
	Task Bullet = iota
	Completed
	MovedCollection
	MovedFuture
	Irrelevant
	Note
	Event
	Any
	Priority Signifier = iota
	Inspiration
	Investigation
	None
)

func BulletForAlias(alias string) (Bullet, error) {
	for i, g := range DefaultGlyphs() {
		if alias == g.Symbol {
			return Bullet(i), nil
		}
		for _, a := range g.Aliases {
			if strings.EqualFold(strings.ToLower(a), strings.ToLower(alias)) {
				return Bullet(i), nil
			}
		}
	}
	return Any, fmt.Errorf("unknown bullet alias: %s", alias)
}

func (b Bullet) Glyph() Glyph {
	return DefaultGlyphs()[b]
}

func (b Bullet) String() string {
	return b.Glyph().String()
}

func (s Signifier) Glyph() Glyph {
	return DefaultGlyphs()[s]
}

func (s Signifier) String() string {
	return s.Glyph().String()
}
