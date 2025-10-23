// Package collections contains runners for collection management commands.
package collections

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"tableflip.dev/bujo/pkg/app"
	"tableflip.dev/bujo/pkg/collection"
	"tableflip.dev/bujo/pkg/store"
)

// Type configures the parameters for `bujo collections type`.
type Type struct {
	Collection  string
	Type        collection.Type
	Create      bool
	Persistence store.Persistence
}

// Do executes the type assignment.
func (t *Type) Do(ctx context.Context) error {
	collectionName := strings.TrimSpace(t.Collection)
	if collectionName == "" {
		return errors.New("collection name is required")
	}
	if t.Type == "" {
		return errors.New("collection type is required")
	}
	if t.Persistence == nil {
		var err error
		t.Persistence, err = store.Load(nil)
		if err != nil {
			return err
		}
	}

	svc := app.Service{Persistence: t.Persistence}

	exists := false
	for _, meta := range t.Persistence.CollectionsMeta(ctx, "") {
		if meta.Name == collectionName {
			exists = true
			break
		}
	}
	if !exists && !t.Create {
		return fmt.Errorf("collection %q does not exist (use --create to add it)", collectionName)
	}

	if !exists {
		if err := svc.EnsureCollectionOfType(ctx, collectionName, t.Type); err != nil {
			return err
		}
	} else {
		if err := svc.SetCollectionType(ctx, collectionName, t.Type); err != nil {
			return err
		}
	}

	fmt.Printf("Collection %q set to type %s\n", collectionName, t.Type)
	return nil
}
