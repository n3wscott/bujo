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
	k.Key(ctx, glyph.DefaultGlyphs(), false)
	k.Key(ctx, glyph.DefaultGlyphs(), true)

	return nil
}

func (k *Key) Key(ctx context.Context, glyfs []glyph.Glyph, sig bool) {
	tbl := uitable.New()
	tbl.Separator = "  "
	tbl.AddRow(glyph.Bold("Key"), glyph.Bold("Symbol"), glyph.Bold("Meaning"))
	for _, v := range glyfs {
		if sig == v.Signifier {
			tbl.AddRow(v.Key, v.Symbol, v.Meaning)
		}
	}

	if sig {
		_, _ = fmt.Fprintln(color.Output, glyph.Bold(glyph.Underline("\nSignifier")))
	} else {
		_, _ = fmt.Fprintln(color.Output, glyph.Bold(glyph.Underline("\nBullets")))
	}
	_, _ = fmt.Fprintln(color.Output, tbl)
}
