package menu

import "github.com/charmbracelet/bubbles/v2/list"

func NewItem(name string) list.Item {
	return Item(name)
}

type Item string

func (i Item) FilterValue() string { return "" }
