package printers

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/n3wscott/bujo/pkg/entry"
	"math/rand"
	"strings"
	"time"
)

func (pp *PrettyPrint) Calendar(on time.Time, entries ...*entry.Entry) {
	then := time.Date(on.Year(), on.Month(), 1, 1, 0, 0, 0, time.Local)
	pp.PrintMonthLong(then, entries...)
}

func (pp *PrettyPrint) Tracking(entries ...*entry.Entry) {
	now := time.Now()
	pp.PrintMonth(now, entries...)
}

func (pp *PrettyPrint) TrackingYear(entries ...*entry.Entry) {
	now := time.Now()
	now = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.Local)

	for i := 0; i < 12; i++ {
		pp.PrintMonth(now, entries...)
		now = NextMonth(now)
	}
}

const width = len("11 12 13 14 15 16 17") // an example week

func (pp *PrettyPrint) PrintMonth(then time.Time, entries ...*entry.Entry) {
	days := DaysIn(then)

	count := make([]int, days)

	for _, e := range entries {
		if e.Created.SameMonth(then) {
			count[e.Created.Local().Day()-1]++
		}
	}

	pp.PrintMonthCount(then, count)
}

func (pp *PrettyPrint) PrintMonthCount(then time.Time, count []int) {
	d := StartDay(then)

	tf := color.New(color.FgWhite, color.Italic)

	m := then.Month().String()
	mid := (width - len(m)) / 2
	tf.Printf("%s%s%s\n", strings.Repeat(" ", mid), m, strings.Repeat(" ", width-mid-len(m)))

	days := DaysIn(then)

	// Pad out the start of the month.
	for i := time.Sunday; i < d; i++ {
		if i < d {
			fmt.Print("   ")
		}
	}

	l1 := color.New(color.Faint, color.FgWhite)
	l2 := color.New(color.Bold, color.FgHiWhite)

	for i := 0; i < days; i++ {
		if i < len(count) {
			if count[i] == 0 {
				l1.Printf("%2d ", i+1)
			} else {
				l2.Printf("%2d ", i+1)
			}
		} else {
			l1.Printf("%2d ", i+1)
		}

		d++
		if d > time.Saturday {
			d = time.Sunday
			fmt.Print("\n")
		}
	}
	fmt.Print("\n\n")

}

func (pp *PrettyPrint) PrintMonthLong(then time.Time, entries ...*entry.Entry) {
	p := color.New()
	b := color.New(color.Bold)
	i := color.New(color.Italic)
	s := color.New(color.Underline)
	bs := color.New(color.Underline, color.Bold)

	d := StartDay(then)
	hasOpenDueDate := false
	for i := 0; i < DaysIn(then); i++ {
		// TODO: all this logic can get cleaner.
		now := time.Now()
		printer := p

		if now.Month() == then.Local().Month() && now.Year() == then.Year() && now.Local().Day() == i+1 {
			printer = b
		}
		if d == time.Sunday {
			printer = s
			if now.Month() == then.Local().Month() && now.Year() == then.Year() && now.Local().Day() == i+1 {
				printer = bs
			}
		}
		_, _ = printer.Printf("%2d %s", i+1, d.String()[0:1])

		found := false
		for _, e := range entries {
			if found {
				_, _ = p.Print("      ") // space.
			} else {
				_, _ = p.Print("  ") // space.
			}
			if e.On == nil {
				hasOpenDueDate = true
				continue
			}
			if e.On.Year() == then.Local().Year() && e.On.Month() == then.Local().Month() && e.On.Day() == i {
				found = true
				_, _ = p.Printf("%s %s %s\n", e.Signifier.String(), e.Bullet.String(), e.Message)
			}
		}
		d++
		if d > time.Saturday {
			d = time.Sunday
		}
		if !found {
			_, _ = p.Printf("\n")
		}
	}

	if hasOpenDueDate {
		_, _ = i.Printf("\nOpen\n")
		for _, e := range entries {
			if e.On == nil {
				_, _ = p.Printf("%s %s %s\n", e.Signifier.String(), e.Bullet.String(), e.Message)
			}
		}
	}
}

func NextMonth(then time.Time) time.Time {
	return time.Date(then.Local().Year(), then.Local().Month()+1, then.Local().Day(), 1, 0, 0, 0, then.Location())
}

func DaysIn(then time.Time) int {
	return time.Date(then.UTC().Year(), then.UTC().Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func StartDay(then time.Time) time.Weekday {
	return time.Date(then.UTC().Year(), then.UTC().Month(), 1, 1, 0, 0, 0, time.UTC).Weekday()
}

// --- Demo

func (pp *PrettyPrint) PrintMonthDemo(then time.Time) {
	days := DaysIn(then)

	count := make([]int, days)

	for i := 0; i < days; i++ {
		count[i] = rand.Intn(2)
	}

	pp.PrintMonthCount(then, count)
}
