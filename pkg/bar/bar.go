package bar

import (
	"context"
	"fmt"
)

type Bar struct {
	Message string
	Name    string
	Output  string
}

func (b *Bar) Do(ctx context.Context) error {
	switch b.Output {
	case "json":
		fmt.Printf(`"%s, %s"`, b.Message, b.Name)
	default:
		fmt.Printf("%s, %s\n", b.Message, b.Name)
	}
	return nil
}
