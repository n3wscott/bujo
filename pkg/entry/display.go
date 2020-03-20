package entry

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/gosuri/uitable"
	"github.com/n3wscott/bujo/pkg/glyph"
)

func PrettyPrintCollection(entries ...*Entry) {
	if len(entries) == 0 {
		return
	}

	fmt.Println(glyph.Underline(glyph.Bold(entries[0].Title())))

	tbl := uitable.New()
	tbl.Separator = " "

	for _, e := range entries {
		tbl.AddRow(e.Row())
	}
	_, _ = fmt.Fprintln(color.Output, tbl)
}
