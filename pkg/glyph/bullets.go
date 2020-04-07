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

type Bullet string
type Signifier string

// These values are what is stored into the database.
// Do not change unless you are ok with loosing data.
const (
	Task            Bullet = "task"
	Completed       Bullet = "comp"
	MovedCollection Bullet = "movc"
	MovedFuture     Bullet = "movf"
	Irrelevant      Bullet = "irev"
	Note            Bullet = "note"
	Event           Bullet = "evnt"
	Any             Bullet = "any"

	Priority      Signifier = "pri0"
	Inspiration   Signifier = "insp"
	Investigation Signifier = "inst"
	None          Signifier = "none"
)

func DefaultBullets() map[Bullet]Glyph {
	return map[Bullet]Glyph{
		Task: {
			Key:     "+",
			Symbol:  "●",
			Meaning: "task",
			Aliases: []string{"+", "*", "task", "tasks"},
		},
		Completed: {
			Key:     "x",
			Symbol:  "✘",
			Meaning: "task completed",
			Aliases: []string{"x", "completed", "completes", "complete", "done"},
		},
		MovedCollection: {
			Key:     ">",
			Symbol:  "›",
			Meaning: "task moved to collection",
			Aliases: []string{">", "move-collection", "moved-collection"},
		},
		MovedFuture: {
			Key:     "<",
			Symbol:  "‹",
			Meaning: "task moved to future log",
			Aliases: []string{"<", "move-future", "moved-future"},
		},
		Irrelevant: {
			Key:     "~",
			Symbol:  "⦵",
			Meaning: "task irrelevant",
			Aliases: []string{"~", "strike", "strikes", "striked"},
		},
		Note: {
			Key:     "-",
			Symbol:  "⁃",
			Meaning: "note",
			Aliases: []string{"-", "note", "notes", "noted"},
		},
		Event: {
			Key:     "o",
			Symbol:  "○",
			Meaning: "event",
			Aliases: []string{"o", "event", "events"},
		},
		Any: {
			Key:     "",
			Symbol:  "",
			Meaning: "any",
			Aliases: []string{"any"},
		},
	}
}

func DefaultSignifiers() map[Signifier]Glyph {
	return map[Signifier]Glyph{
		Priority: {
			Key:       "*",
			Symbol:    "✷",
			Meaning:   "priority",
			Signifier: true,
		},
		Inspiration: {
			Key:       "!",
			Symbol:    "!",
			Meaning:   "inspiration",
			Signifier: true,
		},
		Investigation: {
			Key:       "?",
			Symbol:    "?",
			Meaning:   "investigation",
			Signifier: true,
		},
		None: {
			Key:       " ",
			Symbol:    " ",
			Meaning:   "none",
			Signifier: true,
		},
	}
}

func (g Glyph) String() string {
	return g.Symbol
}

func BulletForAlias(alias string) (Bullet, error) {
	for i, g := range DefaultBullets() {
		if alias == g.Symbol {
			return i, nil
		}
		for _, a := range g.Aliases {
			if strings.EqualFold(strings.ToLower(a), strings.ToLower(alias)) {
				return i, nil
			}
		}
	}
	return Any, fmt.Errorf("unknown bullet alias: %s", alias)
}

func (b Bullet) Glyph() Glyph {
	return DefaultBullets()[b]
}

func (b Bullet) String() string {
	return b.Glyph().String()
}

func (s Signifier) Glyph() Glyph {
	return DefaultSignifiers()[s]
}

func (s Signifier) String() string {
	return s.Glyph().String()
}
