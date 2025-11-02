package main

import (
	tea "github.com/charmbracelet/bubbletea/v2"

	cachepkg "tableflip.dev/bujo/pkg/tui/cache"
)

type cacheMsg struct {
	payload tea.Msg
}

func cacheListenCmd(cache *cachepkg.Cache) tea.Cmd {
	if cache == nil {
		return nil
	}
	ch := cache.Events()
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return cacheMsg{payload: msg}
	}
}
