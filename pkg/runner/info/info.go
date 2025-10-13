// Package info implements the runner for displaying collection metadata.
package info

import (
	"context"
	"errors"
	"fmt"
	"os"
	"tableflip.dev/bujo/pkg/store"
)

// Info prints information about configuration and collections.
type Info struct {
	Config      store.Config
	Persistence store.Persistence
}

// Do outputs config paths and discovered collections.
func (n *Info) Do(ctx context.Context) error {

	if override := os.Getenv("BUJO_CONFIG_PATH"); override != "" {
		fmt.Println("BUJO_CONFIG_PATH found on env, using ", override)
	} else {
		fmt.Println("BUJO_CONFIG_PATH env var not set")
	}

	if n.Config == nil {
		var err error
		n.Config, err = store.LoadConfig()
		if err != nil {
			return err
		}
	}

	fmt.Println("Config.path: ", n.Config.BasePath())

	if n.Persistence == nil {
		return errors.New("failed to create persistence object")
	}

	fmt.Printf("Collections:\n")
	foundCollections := 0
	for _, k := range n.Persistence.Collections(ctx, "") {
		fmt.Printf("  %s\n", k)
		foundCollections++
	}

	if foundCollections == 0 {
		fmt.Printf("  %s\n", "no collections")
	}

	return nil
}
