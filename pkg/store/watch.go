package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// EventType describes the nature of a persistence change notification.
type EventType int

const (
	// EventCollectionChanged indicates the set of entries for the given
	// collection changed (added, edited, or removed entries).
	EventCollectionChanged EventType = iota

	// EventCollectionsInvalidated signals that the collection catalog itself
	// changed (e.g. a new collection was added or removed) and callers should
	// refresh their full view.
	EventCollectionsInvalidated
)

// Event is emitted by Persistence.Watch when underlying storage changes.
type Event struct {
	Type       EventType
	Collection string
}

// Watch streams change events until ctx is cancelled. Callers should drain the
// returned channel to avoid blocking the watcher. The channel is closed once
// ctx is done or the watcher encounters an unrecoverable error.
func (p *persistence) Watch(ctx context.Context) (<-chan Event, error) {
	if p.basePath == "" {
		return nil, errors.New("store: persistence base path unknown")
	}

	if err := os.MkdirAll(p.basePath, 0o755); err != nil {
		return nil, fmt.Errorf("store: ensure base path: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("store: create watcher: %w", err)
	}
	var closeOnce sync.Once
	closeWatcher := func() {
		closeOnce.Do(func() {
			if err := watcher.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "store: watcher close: %v\n", err)
			}
		})
	}

	dirs, err := collectDirs(p.basePath)
	if err != nil {
		closeWatcher()
		return nil, fmt.Errorf("store: enumerate directories: %w", err)
	}

	for _, dir := range dirs {
		if err := watcher.Add(dir); err != nil {
			closeWatcher()
			return nil, fmt.Errorf("store: watch %s: %w", dir, err)
		}
	}

	events := make(chan Event, 64)

	go func() {
		defer close(events)
		defer closeWatcher()

		// Track directories we already watch so we can add new ones at runtime
		// without duplicating watches.
		watched := make(map[string]struct{}, len(dirs))
		for _, dir := range dirs {
			watched[dir] = struct{}{}
		}

		send := func(ev Event) {
			select {
			case events <- ev:
			default:
				// Drop events if the consumer is not ready; a subsequent
				// refresh will pick up the changes and keeps the UI from
				// stalling. This keeps filesystem storms from blocking the
				// watcher goroutine.
			}
		}

		throttle := newEventThrottle(100 * time.Millisecond)
		defer throttle.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				// Surface watcher errors as a full refresh to keep clients in
				// sync even if we cannot classify the change precisely.
				throttle.Enqueue(Event{Type: EventCollectionsInvalidated}, send)
				_ = err // log? keep silent per CLI guidance.
			case evt, ok := <-watcher.Events:
				if !ok {
					return
				}

				if evt.Op&fsnotify.Create == fsnotify.Create {
					// If a new directory appears, start watching it to capture
					// subsequent file writes.
					if info, err := os.Stat(evt.Name); err == nil && info.IsDir() {
						absDir := filepath.Clean(evt.Name)
						if _, found := watched[absDir]; !found {
							if err := watcher.Add(absDir); err != nil {
								fmt.Fprintf(os.Stderr, "store: watch %s: %v\n", absDir, err)
							} else {
								watched[absDir] = struct{}{}
							}
						}
						// Directory creation likely corresponds to a new
						// collection bucket, so issue a catalog refresh.
						throttle.Enqueue(Event{Type: EventCollectionsInvalidated}, send)
						continue
					}
				}

				collection := p.collectionForPath(evt.Name)
				if collection == "" {
					throttle.Enqueue(Event{Type: EventCollectionsInvalidated}, send)
					continue
				}

				throttle.Enqueue(Event{Type: EventCollectionChanged, Collection: collection}, send)
			}
		}
	}()

	return events, nil
}

// collectDirs walks base and returns all directories that should be watched.
func collectDirs(base string) ([]string, error) {
	dirs := []string{base}
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() && path != base {
			dirs = append(dirs, path)
		}
		return nil
	})
	return dirs, err
}

// collectionForPath attempts to derive the logical collection from a diskv path.
func (p *persistence) collectionForPath(path string) string {
	rel, err := filepath.Rel(p.basePath, path)
	if err != nil {
		return ""
	}
	if rel == "." {
		return ""
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	if len(parts) == 0 {
		return ""
	}
	encoded := parts[0]
	if encoded == "" || encoded == collectionsIndexFile {
		return ""
	}
	return fromCollection(encoded)
}

// eventThrottle coalesces rapid change notifications so the UI can redraw once
// per burst of filesystem activity instead of on every single write.
type eventThrottle struct {
	mu      sync.Mutex
	timer   *time.Timer
	pending map[EventType]map[string]struct{}
	delay   time.Duration
}

func newEventThrottle(delay time.Duration) *eventThrottle {
	return &eventThrottle{
		delay:   delay,
		pending: make(map[EventType]map[string]struct{}),
	}
}

func (t *eventThrottle) Enqueue(ev Event, send func(Event)) {
	t.mu.Lock()
	if t.pending[ev.Type] == nil {
		t.pending[ev.Type] = make(map[string]struct{})
	}
	key := ev.Collection
	t.pending[ev.Type][key] = struct{}{}

	if t.timer == nil {
		t.timer = time.AfterFunc(t.delay, func() {
			t.flush(send)
		})
	}
	t.mu.Unlock()
}

func (t *eventThrottle) flush(send func(Event)) {
	t.mu.Lock()
	pending := t.pending
	t.pending = make(map[EventType]map[string]struct{})
	t.timer = nil
	t.mu.Unlock()

	for eventType, collections := range pending {
		if len(collections) == 0 {
			send(Event{Type: eventType})
			continue
		}

		for collection := range collections {
			send(Event{Type: eventType, Collection: collection})
		}
	}
}

func (t *eventThrottle) Stop() {
	t.mu.Lock()
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
	t.mu.Unlock()
}
