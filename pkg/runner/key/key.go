package key

import (
	"context"
	"fmt"
	"github.com/fatih/color"
	"github.com/gosuri/uitable"
	"github.com/n3wscott/bujo/pkg/glyph"
	"sort"
)

type Key struct{}

func (k *Key) Do(ctx context.Context) error {
	_, _ = fmt.Fprintln(color.Output, "")

	bullets := glyph.DefaultBullets()
	bl := make([]glyph.Glyph, 0, len(bullets))
	for _, v := range bullets {
		if v.Printed {
			bl = append(bl, v)
		}
	}
	sort.Sort(glyph.ByOrder(bl))

	k.Key(ctx, bl, false)
	_, _ = fmt.Fprintln(color.Output, "")

	sigs := glyph.DefaultSignifiers()
	sl := make([]glyph.Glyph, 0, len(sigs))
	for _, v := range sigs {
		if v.Printed {
			sl = append(sl, v)
		}
	}
	sort.Sort(glyph.ByOrder(sl))

	k.Key(ctx, sl, true)

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
