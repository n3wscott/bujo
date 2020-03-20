package foo

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"github.com/gosuri/uitable"
)

type Foo struct {
	List   bool
	Output string
}

func (f *Foo) Do(ctx context.Context) error {
	if f.List {
		return f.DoList(ctx)
	}

	return nil
}

func (f *Foo) DoList(ctx context.Context) error {
	foos := []string{"foo1", "foo2", "foo3"}

	switch f.Output {
	case "json":
		b, err := json.Marshal(foos)
		if err != nil {
			return err
		}
		fmt.Println(string(b))

	default:
		tbl := uitable.New()
		tbl.Separator = "  "
		tbl.AddRow("Index", "Value")
		for k, v := range foos {
			tbl.AddRow(k, v)
		}
		_, _ = fmt.Fprintln(color.Output, tbl)
	}

	return nil
}
