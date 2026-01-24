package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"

	"tableflip.dev/bujo/pkg/store"
	cachepkg "tableflip.dev/bujo/pkg/tui/cache"
	"tableflip.dev/bujo/pkg/tui/events"
)

func (m *Model) loadJournalSnapshot() tea.Cmd {
	if m.service == nil {
		m.journalError = fmt.Errorf("service unavailable")
		return nil
	}
	m.loadingJournal = true
	svc := m.service
	return func() tea.Msg {
		snapshot, err := cachepkg.BuildSnapshotWithClock(context.Background(), svc, m.clock)
		return journalLoadedMsg{snapshot: snapshot, err: err}
	}
}

func (m *Model) scheduleDayCheck() tea.Cmd {
	return tea.Tick(dayCheckInterval, func(time.Time) tea.Msg {
		return dayCheckMsg{}
	})
}

func (m *Model) refreshToday(now time.Time) {
	day := startOfDay(now)
	if !m.today.IsZero() && m.today.Equal(day) {
		return
	}
	m.today = day
	if m.journalNav != nil {
		m.journalNav.SetNow(now)
	}
	m.layoutContent()
}

func cacheListenCmd(cache *cachepkg.Cache) tea.Cmd {
	if cache == nil {
		return nil
	}
	ch := cache.Events()
	component := cache.ComponentID()
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		if msg == nil {
			return nil
		}
		return events.ChildMsg{From: component, Msg: msg}
	}
}

func startWatchCmd(parent context.Context, svc JournalService) tea.Cmd {
	if svc == nil || parent == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(parent)
		ch, err := svc.Watch(ctx)
		if err != nil {
			cancel()
			return watchStartedMsg{err: err}
		}
		return watchStartedMsg{ch: ch, cancel: cancel}
	}
}

func (m *Model) waitForWatch() tea.Cmd {
	if m.watchCh == nil {
		return nil
	}
	ch := m.watchCh
	return func() tea.Msg {
		if ev, ok := <-ch; ok {
			return watchEventMsg{event: ev}
		}
		return watchStoppedMsg{}
	}
}

func (m *Model) stopWatch() {
	if m.watchCancel != nil {
		m.watchCancel()
		m.watchCancel = nil
	}
	m.watchCh = nil
}

func (m *Model) handleWatchEvent(ev store.Event) tea.Cmd {
	switch ev.Type {
	case store.EventCollectionChanged:
		if strings.HasPrefix(ev.Collection, "fromCollection:") {
			return nil
		}
		return m.collectionSyncCmd(ev.Collection)
	case store.EventCollectionsInvalidated:
		return m.snapshotSyncCmd()
	default:
		if strings.HasPrefix(ev.Collection, "fromCollection:") {
			return nil
		}
		return m.snapshotSyncCmd()
	}
}

func (m *Model) collectionSyncCmd(collectionID string) tea.Cmd {
	cache := m.journalCache
	if cache == nil {
		return nil
	}
	return func() tea.Msg {
		if err := cache.SyncCollection(context.Background(), collectionID); err != nil {
			return watchErrorMsg{err: err}
		}
		return nil
	}
}

func (m *Model) snapshotSyncCmd() tea.Cmd {
	cache := m.journalCache
	svc := m.service
	if cache == nil || svc == nil {
		return nil
	}
	return func() tea.Msg {
		snapshot, err := cachepkg.BuildSnapshotWithClock(context.Background(), svc, m.clock)
		if err != nil {
			return watchErrorMsg{err: err}
		}
		cache.ApplySnapshot(snapshot)
		return nil
	}
}
