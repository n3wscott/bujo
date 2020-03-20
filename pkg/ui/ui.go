package ui

import (
	"context"
	"github.com/marcusolsson/tui-go"
	"github.com/n3wscott/bujo/pkg/entry"
	"strings"
)

func Do(ctx context.Context, entries ...*entry.Entry) error {
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
	status.SetPermanentText(`Use left️ or right arrows to navigate, ESC or 'q' to QUIT`)

	collection := tui.NewVBox(cTable)

	collection.SetTitle(entries[0].Collection)
	collection.SetBorder(true)
	collection.SetSizePolicy(tui.Expanding, tui.Maximum)

	selector := tui.NewHBox(index, collection) // tui.NewSpacer(),

	root := tui.NewVBox(
		selector,
		tui.NewSpacer(),
		status,
	)

	ui, err := tui.New(root)
	if err != nil {
		return err
	}

	d := impl{
		Entries:        entries,
		indexes:        iTable,
		indexTitle:     "index",
		indexView:      index,
		collection:     cTable,
		collectionView: collection,
	}
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

type impl struct {
	Entries []*entry.Entry

	dirty string
	index []string

	indexes    *tui.Table
	indexTitle string
	indexView  *tui.Box

	collection      *tui.Table
	collectionView  *tui.Box
	collectionTitle string
}

func (d *impl) focusIndex() {
	d.indexes.SetFocused(true)
	d.indexView.SetTitle(strings.ToUpper(d.indexTitle))

	d.collection.SetFocused(false)
	d.collectionView.SetTitle("")
}

func (d *impl) focusCollection() {
	d.indexes.SetFocused(false)
	d.indexView.SetTitle(d.indexTitle)

	d.collection.SetFocused(true)
	d.collectionView.SetTitle(d.collectionTitle)
}

func (d *impl) populateIndex() {
	d.indexes.RemoveRows()
	d.indexes.Select(0)

	i := make(map[string]bool, 0)
	for _, v := range d.Entries {
		if _, ok := i[v.Collection]; !ok {
			i[v.Collection] = true
		}
	}

	d.index = make([]string, 0, len(i))
	for k, _ := range i {
		d.index = append(d.index, k)
		d.indexes.AppendRow(tui.NewLabel(k))
	}
}

func (d *impl) populateCollection() {
	selected := ""
	if d.indexes.Selected() >= 0 {
		selected = d.index[d.indexes.Selected()]
	}

	if d.dirty != selected {
		d.collection.RemoveRows()

		d.collectionTitle = selected

		for _, v := range d.Entries {
			if selected == v.Collection {
				d.collection.AppendRow(tui.NewLabel(v.String()))
			}
		}
		d.dirty = selected
	}
}
