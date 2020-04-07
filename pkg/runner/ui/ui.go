package ui

import (
	"context"
	"fmt"
	"github.com/marcusolsson/tui-go"
	"github.com/n3wscott/bujo/pkg/entry"
	"github.com/n3wscott/bujo/pkg/glyph"
	"github.com/n3wscott/bujo/pkg/store"
	"strings"
)

type UI struct {
	Persistence store.Persistence

	cache map[string][]*entry.Entry

	dirty string
	index []string

	indexes    *tui.Table
	indexTitle string
	indexView  *tui.Box

	collection      *tui.Table
	collectionView  *tui.Box
	collectionTitle string
}

func (d *UI) Do(ctx context.Context) error {
	iTable := tui.NewTable(1, 0)

	index := tui.NewVBox(
		iTable,
		tui.NewSpacer(),
	)
	index.SetBorder(true)
	index.SetSizePolicy(tui.Preferred, tui.Expanding)
	index.SetBorder(true)

	cTable := tui.NewTable(1, 0)
	cTable.SetFocused(true)

	cTable.SetSizePolicy(tui.Expanding, tui.Maximum)

	status := tui.NewStatusBar("")
	status.SetPermanentText(`Use left️ or right arrows to navigate, 'k' for key, ESC or 'q' to QUIT`)

	collection := tui.NewVBox(cTable)
	collection.SetBorder(true)
	collection.SetSizePolicy(tui.Expanding, tui.Maximum)

	selector := tui.NewHBox(index, collection) // tui.NewSpacer(),

	root := tui.NewVBox(
		selector,
		tui.NewSpacer(),
		status,
	)

	key := keyUI()
	key.SetBorder(true)
	key.SetTitle("key")

	popup := tui.NewVBox(
		tui.NewHBox(key, tui.NewSpacer()),
		tui.NewSpacer(),
		status,
	)

	ui, err := tui.New(root)
	if err != nil {
		return err
	}

	d.indexes = iTable
	d.indexTitle = "index"
	d.indexView = index
	d.collection = cTable
	d.collectionView = collection
	d.cache = d.Persistence.MapAll(ctx)

	d.populateIndex()

	cTable.OnItemActivated(func(t *tui.Table) {
		//if t.Selected() == 0 {
		//	impl.Quit()
		//	fmt.Printf("no selection; context unchanged\n")
		//	return
		//}
		//_, err := cmd(fmt.Sprintf("kubectl config use-context %s", cfg.Contexts[t.Selected()-1].Name))
		//if err != nil {
		//	panic(err)
		//}
		//impl.Quit()
		//fmt.Printf("selected %s\n", cfg.Contexts[t.Selected()-1].Name)
		// TODO
	})

	iTable.OnSelectionChanged(func(table *tui.Table) {
		d.populateCollection()
	})

	isKey := false
	ui.SetKeybinding("k", func() {
		if isKey {
			ui.SetWidget(root)
			isKey = false
		} else {
			ui.SetWidget(popup)
			isKey = true
		}
	})

	ui.SetKeybinding("Left", func() {
		d.focusIndex()
	})

	ui.SetKeybinding("Right", func() {
		d.focusCollection()
	})

	ui.SetKeybinding("Esc", func() { ui.Quit() })
	ui.SetKeybinding("q", func() { ui.Quit() })

	d.populateCollection()
	d.focusCollection()

	if err := ui.Run(); err != nil {
		return err
	}
	return nil
}

func (d *UI) focusIndex() {
	d.indexes.SetFocused(true)
	d.indexView.SetTitle(strings.ToUpper(d.indexTitle))

	d.collection.SetFocused(false)
	d.collectionView.SetTitle("")
}

func (d *UI) focusCollection() {
	d.indexes.SetFocused(false)
	d.indexView.SetTitle(d.indexTitle)

	d.collection.SetFocused(true)
	d.collectionView.SetTitle(d.collectionTitle)
}

func (d *UI) populateIndex() {
	d.indexes.RemoveRows()
	d.indexes.Select(0)

	i := make(map[string]bool, 0)
	for c, _ := range d.cache {
		if _, ok := i[c]; !ok {
			i[c] = true
		}
	}

	d.index = make([]string, 0, len(i))
	for k, _ := range i {
		d.index = append(d.index, k)
		d.indexes.AppendRow(tui.NewLabel(k))
	}
}

func (d *UI) populateCollection() {
	selected := ""
	if d.indexes.Selected() >= 0 {
		selected = d.index[d.indexes.Selected()]
	}

	if d.dirty != selected {
		d.collection.RemoveRows()
		d.collectionTitle = selected
		if col, ok := d.cache[selected]; ok {
			for _, e := range col {
				d.collection.AppendRow(tui.NewLabel(e.String()))
			}
		}
		d.dirty = selected
	}
}

func keyUI() *tui.Box {
	bull := make([]tui.Widget, 0)
	sigs := make([]tui.Widget, 0)

	bull = append(bull, tui.NewLabel("Bullets"))
	sigs = append(sigs, tui.NewLabel("Signifiers"))

	for _, v := range glyph.DefaultBullets() {
		bull = append(bull, tui.NewLabel(fmt.Sprintf("%s  %s", v.Symbol, v.Meaning)))
	}
	for _, v := range glyph.DefaultSignifiers() {
		sigs = append(sigs, tui.NewLabel(fmt.Sprintf("%s  %s", v.Symbol, v.Meaning)))
	}
	bull = append(bull, tui.NewLabel(""))
	sigs = append(sigs, tui.NewSpacer())

	return tui.NewVBox(append(bull, sigs...)...)
}
