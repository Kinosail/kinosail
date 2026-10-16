package catalog

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"maps"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	libraryWatchDebounce     = 2 * time.Second
	libraryWatchStability    = 2 * time.Second
	libraryWatchRetry        = 30 * time.Second
	libraryWatchBackoffSteps = 5
	libraryPollInterval      = time.Minute
)

func (index *Index) Watch(ctx context.Context) {
	select {
	case <-index.watchChange:
	default:
	}
	go index.poll(ctx)
	go func() {
		failures := 0
		for ctx.Err() == nil {
			err := index.watch(ctx)
			if ctx.Err() != nil {
				return
			}
			if err == nil {
				continue
			}
			var retry time.Duration
			failures, retry = index.watchFailed(err, failures)
			select {
			case <-ctx.Done():
				return
			case <-index.watchChange:
			case <-time.After(retry):
			}
		}
	}()
}

func (index *Index) watchFailed(err error, failures int) (int, time.Duration) {
	healthy, _ := index.Monitoring()
	index.setMonitoring(false, err)
	if healthy {
		failures = 0
		index.RequestRefresh()
	}
	failures++
	retry := index.watchRetry << min(failures-1, libraryWatchBackoffSteps)
	if failures == 1 {
		slog.Warn("real-time Library monitoring failed; polling Library folders until it recovers", "operation", "library.watch", "failure_class", watchFailureClass(err), "retry", retry)
	} else {
		slog.Debug("real-time Library monitoring is still unavailable", "operation", "library.watch", "failure_class", watchFailureClass(err), "attempt", failures, "retry", retry)
	}
	return failures, retry
}

func watchFailureClass(err error) string {
	switch {
	case errors.Is(err, syscall.ENOSPC):
		return "watch_limit"
	case errors.Is(err, syscall.EMFILE):
		return "open_file_limit"
	case errors.Is(err, fs.ErrPermission):
		return "permission"
	case errors.Is(err, fs.ErrNotExist):
		return "missing"
	default:
		return "watcher"
	}
}

func (index *Index) poll(ctx context.Context) {
	ticker := time.NewTicker(index.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		roots, within := index.pollRoots()
		if len(roots) == 0 {
			continue
		}
		current := index.changedDirectories(ctx, roots, within)
		for len(current) > 0 {
			timer := time.NewTimer(index.watchStability)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			next := index.changedDirectories(ctx, roots, within)
			if maps.Equal(current, next) {
				index.RequestChanges(slices.Collect(maps.Keys(current)))
				break
			}
			current = next
		}
	}
}

func (index *Index) pollRoots() ([]ScanRoot, map[ScanRoot][]string) {
	roots := index.Roots()
	if watching, _ := index.Monitoring(); !watching {
		return roots, nil
	}
	polled, local := make([]ScanRoot, 0, len(roots)), make([]ScanRoot, 0, len(roots))
	for _, root := range roots {
		if networkFilesystem(root.Path) {
			polled = append(polled, root)
		} else {
			local = append(local, root)
		}
	}
	within := nestedNetworkMounts(local)
	for _, root := range local {
		if len(within[root]) > 0 {
			polled = append(polled, root)
		}
	}
	return polled, within
}

func (index *Index) watchChanges(ctx context.Context) error {
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
	index.requestVerify()

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
	changed := slices.Collect(maps.Keys(cycle.dirty))
	cycle.dirty, cycle.sample = make(map[string]struct{}), nil
	index.RequestChanges(changed)
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

func (index *Index) Monitoring() (bool, error) {
	index.mu.RLock()
	defer index.mu.RUnlock()
	return index.watching, index.watchErr
}
