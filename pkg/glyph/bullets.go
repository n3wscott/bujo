package glyph

import "fmt"

type Glyph struct {
	Key       string
	Symbol    string
	Meaning   string
	Signifier bool
}

const (
	escape        = "\x1b"
	resetCode     = 0
	boldCode      = 1
	italicCode    = 3
	underlineCode = 4
	strikeCode    = 9
)

func Strike(in string) string {
	return fmt.Sprintf("%s[%dm%s%s[%dm", escape, strikeCode, in, escape, resetCode)
}

func Bold(in string) string {
	return fmt.Sprintf("%s[%dm%s%s[%dm", escape, boldCode, in, escape, resetCode)
}

func Underline(in string) string {
	return fmt.Sprintf("%s[%dm%s%s[%dm", escape, underlineCode, in, escape, resetCode)
}

func DefaultGlyphs() []Glyph {
	g := make([]Glyph, 0, 9)

	g = append(g, Glyph{
		Key:       "+",
		Symbol:    "●",
		Meaning:   "task",
		Signifier: false,
	}, Glyph{
		Key:       "x",
		Symbol:    "✘",
		Meaning:   "task completed",
		Signifier: false,
	}, Glyph{
		Key:       ">",
		Symbol:    "›",
		Meaning:   "task moved to collection",
		Signifier: false,
	}, Glyph{
		Key:     "<",
		Symbol:  "‹",
		Meaning: "task moved to future log",
	}, Glyph{
		Key:    "~",
		Symbol: "⦵",
		//Meaning: Strike("task irrelevant"),
		Meaning: "task irrelevant", // the terminal escaping does not work inside the tui
	}, Glyph{
		Key:     "-",
		Symbol:  "⁃",
		Meaning: "note",
	}, Glyph{
		Key:     "o",
		Symbol:  "○",
		Meaning: "event",
	}, Glyph{
		Key:     "",
		Symbol:  "",
		Meaning: "any",
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
