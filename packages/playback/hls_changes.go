package playback

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	hlsPollInterval = 25 * time.Millisecond
	hlsWatchWarning = time.Minute
)

var hlsWatchWarned atomic.Int64

type hlsChanges struct {
	changed chan struct{}
	watcher *fsnotify.Watcher
	stop    chan struct{}
	joined  chan struct{}
	closed  sync.Once
}

func watchHLSChanges(ctx context.Context, directories []string) *hlsChanges {
	changes := &hlsChanges{changed: make(chan struct{}, 1), stop: make(chan struct{}), joined: make(chan struct{})}
	var pending []string
	if len(directories) > 0 {
		watcher, err := fsnotify.NewWatcher()
		if err == nil {
			changes.watcher = watcher
			pending = changes.watch(slices.Clone(directories))
		} else {
			warnHLSWatchUnavailable(ctx, err)
		}
	}
	go changes.run(pending)
	return changes
}

func warnHLSWatchUnavailable(ctx context.Context, err error) {
	now, last := time.Now().UnixNano(), hlsWatchWarned.Load()
	if now-last < int64(hlsWatchWarning) || !hlsWatchWarned.CompareAndSwap(last, now) {
		return
	}
	class := "unavailable"
	if errors.Is(err, syscall.EMFILE) || errors.Is(err, syscall.ENFILE) {
		class = "instance-limit"
	}
	slog.WarnContext(ctx, "HLS change watch unavailable", "diagnostic", "[PLAYBACK-HLS]", "outcome", "polling", "failure_class", class)
}

func (changes *hlsChanges) run(pending []string) {
	defer close(changes.joined)
	ticker := time.NewTicker(hlsPollInterval)
	defer ticker.Stop()
	var events <-chan fsnotify.Event
	var failures <-chan error
	if changes.watcher != nil {
		events, failures = changes.watcher.Events, changes.watcher.Errors
	}
	for {
		select {
		case <-changes.stop:
			return
		case event, open := <-events:
			if !open {
				return
			}
			watching := len(pending)
			pending = changes.watch(pending)
			if !event.Has(fsnotify.Rename) && len(pending) == watching {
				continue
			}
		case _, open := <-failures:
			if !open {
				return
			}
		case <-ticker.C:
			pending = changes.watch(pending)
		}
		select {
		case changes.changed <- struct{}{}:
		default:
		}
	}
}

func (changes *hlsChanges) watch(pending []string) []string {
	if changes.watcher == nil || len(pending) == 0 {
		return pending
	}
	remaining := pending[:0]
	for _, directory := range pending {
		if changes.watcher.Add(directory) == nil {
			continue
		}
		remaining = append(remaining, directory)
		_ = changes.watcher.Add(filepath.Dir(directory))
	}
	return remaining
}

func (changes *hlsChanges) Close() {
	changes.closed.Do(func() {
		close(changes.stop)
		<-changes.joined
		if changes.watcher != nil {
			go func() { _ = changes.watcher.Close() }()
		}
	})
}
