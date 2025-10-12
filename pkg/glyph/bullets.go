package glyph

import (
	"fmt"
	"strings"
)

type Glyph struct {
	Symbol    string
	Meaning   string
	Noun      string
	Aliases   []string
	Signifier bool
	Printed   bool
	Order     int
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
	Occurrence      Bullet = "occr"

	Priority      Signifier = "pri0"
	Inspiration   Signifier = "insp"
	Investigation Signifier = "inst"
	None          Signifier = "none"
)

func DefaultBullets() map[Bullet]Glyph {
	return map[Bullet]Glyph{
		Task: {
			Symbol:  "⦁",
			Meaning: "task",
			Noun:    "tasks",
			Aliases: []string{"+", "*", "task", "tasks"},
			Printed: true,
			Order:   1,
		},
		Completed: {
			Symbol:  "✘",
			Meaning: "task completed",
			Noun:    "completed",
			Aliases: []string{"x", "completed", "completes", "complete", "done"},
			Printed: true,
			Order:   2,
		},
		MovedCollection: {
			Symbol:  "›",
			Meaning: "task moved to collection",
			Noun:    "moved-collection",
			Aliases: []string{">", "move-collection", "moved-collection"},
			Printed: true,
			Order:   3,
		},
		MovedFuture: {
			Symbol:  "‹",
			Meaning: "task moved to future log",
			Noun:    "moved-future",
			Aliases: []string{"<", "move-future", "moved-future"},
			Printed: true,
			Order:   4,
		},
		Irrelevant: {
			Symbol:  "⦵",
			Meaning: "task irrelevant",
			Noun:    "striked",
			Aliases: []string{"~", "strike", "strikes", "striked"},
			Printed: true,
			Order:   5,
		},
		Note: {
			Symbol:  "⁃",
			Meaning: "note",
			Noun:    "notes",
			Aliases: []string{"-", "note", "notes", "noted"},
			Printed: true,
			Order:   6,
		},
		Event: {
			Symbol:  "○",
			Meaning: "event",
			Noun:    "events",
			Aliases: []string{"o", "event", "events"},
			Printed: true,
			Order:   7,
		},
		Any: {
			Meaning: "any",
			Noun:    "any",
			Aliases: []string{"any"},
			Printed: false,
		},
		Occurrence: {
			Symbol:  "✔︎",
			Meaning: "Tracked occurrence",
			Noun:    "tracked",
			Aliases: []string{"track", "tracked", "occurrence"},
			Printed: false,
		},
	}
}

func DefaultSignifiers() map[Signifier]Glyph {
	return map[Signifier]Glyph{
		Priority: {
			Symbol:    "✷",
			Meaning:   "priority",
			Signifier: true,
			Printed:   true,
			Order:     1,
		},
		Inspiration: {
			Symbol:    "!",
			Meaning:   "inspiration",
			Signifier: true,
			Printed:   true,
			Order:     2,
		},
		Investigation: {
			Symbol:    "?",
			Meaning:   "investigation",
			Signifier: true,
			Printed:   true,
			Order:     3,
		},
		None: {
			Symbol:    " ",
			Meaning:   "none",
			Signifier: true,
			Printed:   false,
		},
	}
}

func (g Glyph) String() string {
	return g.Symbol
}

// Sort by order using: sort.Sort(glyph.ByOrder(glyphs))

type ByOrder []Glyph

func (o ByOrder) Len() int           { return len(o) }
func (o ByOrder) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o ByOrder) Less(i, j int) bool { return o[i].Order < o[j].Order }

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
