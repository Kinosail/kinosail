package playback

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	hlsWatchWarning = time.Minute
	hlsWatchRetry   = time.Minute
)

var hlsWatchWarned atomic.Int64

var hlsWatches = newHLSWatchHub()

type hlsWatchHub struct {
	mu       sync.Mutex
	watcher  *fsnotify.Watcher
	failedAt time.Time
	watched  map[string]map[*hlsChanges]int
	pending  map[string]map[*hlsChanges]int
}

func newHLSWatchHub() *hlsWatchHub {
	return &hlsWatchHub{watched: map[string]map[*hlsChanges]int{}, pending: map[string]map[*hlsChanges]int{}}
}

func (hub *hlsWatchHub) subscribe(ctx context.Context, changes *hlsChanges, directory string, parent bool) bool {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if !hub.ready(ctx) {
		return false
	}
	if _, watched := hub.watched[directory]; !watched {
		if err := hub.watcher.Add(directory); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				warnHLSWatchUnavailable(ctx, err)
			}
			return false
		}
		hub.watched[directory] = map[*hlsChanges]int{}
	}
	hub.watched[directory][changes]++
	if parent {
		hub.count(hub.pending, directory, changes, 1)
	}
	return true
}

func (hub *hlsWatchHub) unsubscribe(changes *hlsChanges, directory string, parent bool) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if parent {
		hub.count(hub.pending, directory, changes, -1)
	}
	subscribers, watched := hub.watched[directory]
	if !watched || subscribers[changes] == 0 {
		return
	}
	hub.count(hub.watched, directory, changes, -1)
	if _, remaining := hub.watched[directory]; !remaining {
		_ = hub.watcher.Remove(directory)
	}
}

func (hub *hlsWatchHub) count(table map[string]map[*hlsChanges]int, directory string, changes *hlsChanges, delta int) {
	subscribers := table[directory]
	if subscribers == nil {
		if delta < 0 {
			return
		}
		subscribers = map[*hlsChanges]int{}
		table[directory] = subscribers
	}
	if subscribers[changes] += delta; subscribers[changes] <= 0 {
		delete(subscribers, changes)
	}
	if len(subscribers) == 0 {
		delete(table, directory)
	}
}

func (hub *hlsWatchHub) ready(ctx context.Context) bool {
	if hub.watcher != nil {
		return true
	}
	if !hub.failedAt.IsZero() && time.Since(hub.failedAt) < hlsWatchRetry {
		return false
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		hub.failedAt = time.Now()
		warnHLSWatchUnavailable(ctx, err)
		return false
	}
	hub.watcher, hub.failedAt = watcher, time.Time{}
	go hub.dispatch(watcher)
	return true
}

func (hub *hlsWatchHub) dispatch(watcher *fsnotify.Watcher) {
	for {
		select {
		case event, open := <-watcher.Events:
			if !open {
				return
			}
			hub.deliver(event)
		case _, open := <-watcher.Errors:
			if !open {
				return
			}
			hub.mu.Lock()
			for _, subscribers := range hub.watched {
				wakeHLSSubscribers(subscribers, false)
			}
			hub.mu.Unlock()
		}
	}
}

func (hub *hlsWatchHub) deliver(event fsnotify.Event) {
	renamed, created := event.Has(fsnotify.Rename), event.Has(fsnotify.Create)
	if !renamed && !created && !event.Has(fsnotify.Remove) {
		return
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if subscribers, watched := hub.watched[event.Name]; watched && !created {
		delete(hub.watched, event.Name)
		_ = hub.watcher.Remove(event.Name)
		wakeHLSSubscribers(subscribers, false)
	}
	directory := filepath.Dir(event.Name)
	if renamed {
		wakeHLSSubscribers(hub.watched[directory], false)
	} else if created {
		wakeHLSSubscribers(hub.pending[directory], true)
	}
}

func wakeHLSSubscribers(subscribers map[*hlsChanges]int, rewatch bool) {
	for changes := range subscribers {
		signal := changes.wake
		if rewatch {
			signal = changes.rewatch
		}
		select {
		case signal <- struct{}{}:
		default:
		}
	}
}

func warnHLSWatchUnavailable(ctx context.Context, err error) {
	now, last := time.Now().UnixNano(), hlsWatchWarned.Load()
	if now-last < int64(hlsWatchWarning) || !hlsWatchWarned.CompareAndSwap(last, now) {
		return
	}
	class := "unavailable"
	switch {
	case errors.Is(err, syscall.EMFILE) || errors.Is(err, syscall.ENFILE):
		class = "instance-limit"
	case errors.Is(err, syscall.ENOSPC):
		class = "watch-limit"
	}
	slog.WarnContext(ctx, "HLS change watch unavailable", "diagnostic", "[PLAYBACK-HLS]", "outcome", "polling", "failure_class", class)
}
