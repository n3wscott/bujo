package printers

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/n3wscott/bujo/pkg/entry"
	"strings"
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
}
