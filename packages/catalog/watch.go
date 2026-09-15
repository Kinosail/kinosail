package catalog

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"maps"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	libraryWatchDebounce  = 2 * time.Second
	libraryWatchStability = 2 * time.Second
	libraryWatchRetry     = 30 * time.Second
	libraryPollInterval   = 30 * time.Second
)

// Watch monitors configured roots and requests stable, coalesced refreshes.
func (index *Index) Watch(ctx context.Context) {
	select {
	case <-index.watchChange:
	default:
	}
	go index.poll(ctx)
	go func() {
		for ctx.Err() == nil {
			err := index.watch(ctx)
			if ctx.Err() != nil {
				return
			}
			if err == nil {
				continue
			}
			index.setMonitoring(false, err)
			index.RequestRefresh()
			slog.Warn("real-time Library monitoring failed; scheduled scans remain active", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-index.watchChange:
			case <-time.After(index.watchRetry):
			}
		}
	}()
}

func (index *Index) poll(ctx context.Context) { //nolint:gocognit // One loop owns polling, stability checks, refresh requests, and cancellation.
	previous, _ := index.snapshot(index.rootSet())
	ticker := time.NewTicker(index.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		current, err := index.snapshot(index.rootSet())
		if err != nil {
			slog.Warn("Library polling failed; retrying automatically", "error", err)
			continue
		}
		for !maps.Equal(previous, current) {
			timer := time.NewTimer(index.watchStability)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			next, err := index.snapshot(index.rootSet())
			if err != nil {
				slog.Warn("Library polling failed; retrying automatically", "error", err)
				break
			}
			if maps.Equal(current, next) {
				index.RequestRefresh()
			}
			previous, current = current, next
		}
	}
}

func (index *Index) rootSet() map[string]struct{} {
	roots := index.Roots()
	result := make(map[string]struct{}, len(roots))
	for _, root := range roots {
		result[root.Path] = struct{}{}
	}
	return result
}

func (index *Index) watchChanges(ctx context.Context) error { //nolint:cyclop,gocognit // Scores of 14 and 19 remain below the enforced repository ceiling of 22.
	watcher, err := index.openWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()
	for _, root := range index.Roots() {
		if err := addWatchTree(watcher, root.Path); err != nil {
			return err
		}
	}
	index.setMonitoring(true, nil)
	index.RequestRefresh() // Close the gap between the startup scan and watcher registration.

	cycle := newWatchCycle()
	defer cycle.close()
	dispatch := watchDispatch{index: index, watcher: watcher, cycle: cycle}
	return watchEvents(ctx, index.watchChange, watcher.Events, watcher.Errors, dispatch.quiet, dispatch.clearQuiet, dispatch.record, dispatch.settle)
}

type watchCycle struct {
	timer  *time.Timer
	quiet  <-chan time.Time
	dirty  map[string]struct{}
	sample map[string]fileStamp
}

func newWatchCycle() *watchCycle {
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	return &watchCycle{timer: timer, dirty: make(map[string]struct{})}
}

func (cycle *watchCycle) close() {
	cycle.timer.Stop()
}

func (cycle *watchCycle) arm(delay time.Duration) {
	cycle.timer.Stop()
	cycle.timer.Reset(delay)
	cycle.quiet = cycle.timer.C
}

func (cycle *watchCycle) record(watcher *fsnotify.Watcher, event fsnotify.Event, delay time.Duration) error {
	if event.Op&fsnotify.Create != 0 {
		if err := addWatchTree(watcher, event.Name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) != 0 {
		cycle.dirty[event.Name] = struct{}{}
		cycle.sample = nil
		cycle.arm(delay)
	}
	return nil
}

func (cycle *watchCycle) settle(index *Index) error {
	current, err := snapshotFiles(cycle.dirty)
	if err != nil {
		return err
	}
	if cycle.sample == nil || !maps.Equal(cycle.sample, current) {
		cycle.sample = current
		cycle.arm(index.watchStability)
		return nil
	}
	cycle.dirty, cycle.sample = make(map[string]struct{}), nil
	index.RequestRefresh()
	return nil
}

type fileStamp struct {
	size     int64
	modified int64
}

func snapshotFiles(paths map[string]struct{}) (map[string]fileStamp, error) {
	state := make(map[string]fileStamp)
	for root := range paths {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			if err != nil || entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
				return err
			}
			info, err := entry.Info()
			if err == nil {
				state[path] = fileStamp{info.Size(), info.ModTime().UnixNano()}
			}
			return err
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return state, nil
}

func addWatchTree(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
}

func (index *Index) signalWatchChange() {
	select {
	case index.watchChange <- struct{}{}:
	default:
	}
}

func (index *Index) setMonitoring(watching bool, err error) {
	index.mu.Lock()
	index.watching, index.watchErr = watching, err
	index.mu.Unlock()
}

// Monitoring reports whether filesystem events are active and their last error.
func (index *Index) Monitoring() (bool, error) {
	index.mu.RLock()
	defer index.mu.RUnlock()
	return index.watching, index.watchErr
}
