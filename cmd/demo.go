package main

import (
	"context"
	"fmt"
	"github.com/n3wscott/bujo/pkg/store"
	"github.com/n3wscott/bujo/pkg/ui"
)

func main() {
	l := ui.StaticDemo()

	p, err := store.Load(nil)
	if err != nil {
		panic(err)
	}

	for _, e := range l {
		if err := p.Store(e); err != nil {
			panic(err)
		}
	}

	for _, e := range p.MapAll(context.Background()) {
		fmt.Println(e.String())
	}

}
