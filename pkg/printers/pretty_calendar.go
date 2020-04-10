package printers

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/n3wscott/bujo/pkg/entry"
	"math/rand"
	"strings"
	"time"
)

func (pp *PrettyPrint) Tracking(entries ...*entry.Entry) {
	now := time.Now()
	pp.PrintMonth(now, entries...)
}

func (pp *PrettyPrint) TrackingYear(entries ...*entry.Entry) {
	now := time.Now()
	now = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 12; i++ {
		pp.PrintMonth(now, entries...)
		now = NextMonth(now)
	}
}

const width = len("11 12 13 14 15 16 17") // an example week

func (pp *PrettyPrint) PrintMonth(then time.Time, entries ...*entry.Entry) {
	days := DaysIn(then)

	count := make([]int, days)

	//for i := 0; i < days; i++ {
	//	count[i] = rand.Intn(2)
	//}

	for _, e := range entries {
		if e.Created.Month() == then.Month() {
			count[e.Created.Day()-1]++
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

func NextMonth(then time.Time) time.Time {
	return time.Date(then.Year(), then.Month()+1, then.Day(), 1, 0, 0, 0, then.Location())
}

func DaysIn(then time.Time) int {
	return time.Date(then.Year(), then.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func StartDay(then time.Time) time.Weekday {
	return time.Date(then.Year(), then.Month(), 1, 1, 0, 0, 0, time.UTC).Weekday()
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
