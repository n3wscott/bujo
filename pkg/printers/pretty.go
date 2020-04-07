package printers

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/n3wscott/bujo/pkg/entry"
	"math/rand"
	"strings"
	"time"
)

type PrettyPrint struct {
	ShowID bool
}

var (
	spacing = strings.Repeat(" ", len("171dff69f8b99dca  "))
)

func (pp *PrettyPrint) Title(title string) {
	t := color.New(color.Bold, color.Underline)

	if pp.ShowID {
		_, _ = t.Print(spacing)
	}
	_, _ = t.Println(title)
}

func (pp *PrettyPrint) Collection(entries ...*entry.Entry) {
	if len(entries) == 0 {
		f := color.New(color.Faint, color.Italic)
		if pp.ShowID {
			_, _ = f.Print(spacing)
		}
		_, _ = f.Print(" none\n\n")
		return
	}

	t := color.New()
	y := color.New(color.FgHiYellow, color.Italic, color.Faint)

	for _, e := range entries {
		if pp.ShowID {
			_, _ = y.Print(e.ID)
			_, _ = y.Print(strings.Repeat(" ", len(spacing)-len(e.ID)))
		}
		_, _ = t.Printf("%s %s %s\n", e.Signifier.String(), e.Bullet.String(), e.Message)
	}
	_, _ = t.Println("")
}

func (pp *PrettyPrint) Tracking(entries ...*entry.Entry) {
	fmt.Printf("TODO\n")

	now := time.Now()
	now = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 12; i++ {
		pp.PrintMonth(now, entries...)
		now = NextMonth(now)
	}
}

func (pp *PrettyPrint) PrintMonth(then time.Time, entries ...*entry.Entry) {
	d := StartDay(then)

	fmt.Println(then.Month().String())

	days := DaysIn(then)

	// Pad out the start of the month.
	for i := time.Sunday; i < d; i++ {
		if i < d {
			fmt.Print("  ")
		}
	}

	l1 := color.New(color.Faint, color.FgWhite)
	l2 := color.New(color.Faint, color.FgWhite)
	l3 := color.New(color.FgWhite)
	l4 := color.New(color.Bold, color.FgHiWhite)

	for i := 0; i < days; i++ {
		switch rand.Intn(4) {
		case 0:
			l1.Print("□ ")
		case 1:
			l2.Print("■ ")
		case 2:
			l3.Print("■ ")
		case 3:
			l4.Print("■ ")
		}

		d++
		if d > time.Saturday {
			d = time.Sunday
			fmt.Print("\n")
		}
	}
	fmt.Print("\n\n")

}

func NextMonth(then time.Time) time.Time {
	return time.Date(then.Year(), then.Month()+1, then.Day(), 1, 0, 0, 0, then.Location())
}

func DaysIn(then time.Time) int {
	return time.Date(then.Year(), then.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func StartDay(then time.Time) time.Weekday {
	return time.Date(then.Year(), then.Month(), 1, 1, 0, 0, 0, time.UTC).Weekday()
}
