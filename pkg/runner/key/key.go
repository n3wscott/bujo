package key

import (
	"context"
	"fmt"
	"github.com/fatih/color"
	"github.com/gosuri/uitable"
	"github.com/n3wscott/bujo/pkg/glyph"
)

type Key struct{}

func (k *Key) Do(ctx context.Context) error {
	_, _ = fmt.Fprintln(color.Output, "")
	k.Key(ctx, glyph.DefaultGlyphs(), false)
	_, _ = fmt.Fprintln(color.Output, "")
	k.Key(ctx, glyph.DefaultGlyphs(), true)

	return nil
}

func (k *Key) Key(ctx context.Context, glyfs []glyph.Glyph, sig bool) {
	bold := color.New(color.Bold)

	tbl := uitable.New()
	tbl.Separator = "  "
	if sig {
		tbl.AddRow(bold.Sprint("Signifiers"), bold.Sprint("Meaning"))
	} else {
		tbl.AddRow(bold.Sprint("Bullets"), bold.Sprint("Meaning"))
	}
	for _, v := range glyfs {
		if sig == v.Signifier {
			tbl.AddRow(v.Symbol, v.Meaning)
		}
	}
	tbl.RightAlign(0)

	_, _ = fmt.Fprintln(color.Output, tbl)
}
