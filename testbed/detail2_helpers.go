package main

import (
	"fmt"
	"strings"

	collectiondetail2 "tableflip.dev/bujo/pkg/tui/components/collectiondetail2"
)

func parseDetail2Mode(value string) (collectiondetail2.Mode, error) {
	mode := strings.ToLower(strings.TrimSpace(value))
	switch mode {
	case "", "continuous":
		return collectiondetail2.ModeContinuous, nil
	case "focused":
		return collectiondetail2.ModeFocused, nil
	default:
		return collectiondetail2.ModeContinuous, fmt.Errorf("unknown mode %q (expected continuous or focused)", value)
	}
}
