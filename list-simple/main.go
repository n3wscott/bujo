package main

import (
	"fmt"
	"os"
	"tableflip.dev/bujo/pkg/ui/menu"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	m := menu.New(
		"Ramen",
		"Tomato Soup",
		"Hamburgers",
		"Cheeseburgers",
		"Currywurst",
		"Okonomiyaki",
		"Pasta",
		"Fillet Mignon",
		"Caviar",
		"Just Wine",
	)

	if err := tea.NewProgram(m).Start(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
