package menu

import (
    "fmt"
    "io"

    "github.com/charmbracelet/bubbles/v2/list"
    tea "github.com/charmbracelet/bubbletea"
)

func Delegate() list.ItemDelegate {
	return &itemDelegate{}
}

type itemDelegate struct{}

func (d itemDelegate) Height() int {
	return 1
}

func (d itemDelegate) Spacing() int {
	return 0
}

func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(Item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s", index+1, i)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s string) string {
			return selectedItemStyle.Render("> " + s)
		}
	}

	_, _ = fmt.Fprintf(w, fn(str))
}
