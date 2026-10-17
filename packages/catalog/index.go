package catalog

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/fsnotify/fsnotify"
)

type Index struct {
	mu                sync.RWMutex
	refreshMu         sync.Mutex
	resolveMu         sync.Mutex
	roots             []ScanRoot
	cache             string
	allowed           atomic.Pointer[allowedRoots]
	current           atomic.Pointer[library.Snapshot]
	titleOrders       map[titleOrderKey]*titleOrder
	titleOrderUse     uint64
	titleOrderVersion uint64
	err               error
	ready             bool
	scanned           time.Time
	interval          time.Duration
	defaultInterval   time.Duration
	reschedule        chan struct{}
	refreshes         chan struct{}
	acquire           func(context.Context) (func(), error)
	decorate          func([]library.Item) []library.Item
	analyzers         []func([]library.Item)
	watching          bool
	watchErr          error
	watchChange       chan struct{}
	watchDebounce     time.Duration
	watchStability    time.Duration
	watchRetry        time.Duration
	pollInterval      time.Duration
	watch             func(context.Context) error
	openWatcher       func() (*fsnotify.Watcher, error)
	lifecycle         context.Context
	scanners          map[ScanRoot]*library.Scanner
	composeMu         sync.Mutex
	analyzeMu         sync.Mutex
	analyzed          uint64
	raw               []library.Item
	enriched          map[string]enrichedItem
	enrich            func(context.Context, []library.Item) []library.Item
	enriches          func(library.Item) bool
	background        bool
	enrichStart       sync.Once
	enrichSignal      chan struct{}
	pendingMu         sync.Mutex
	pending           refreshRequest
}

func NewIndex(ctx context.Context, roots []ScanRoot, cache string, interval time.Duration, acquire func(context.Context) (func(), error)) *Index {
	return NewDecoratedIndex(ctx, roots, cache, interval, acquire, Decoration{})
}

func NewDecoratedIndex(ctx context.Context, roots []ScanRoot, cache string, interval time.Duration, acquire func(context.Context) (func(), error), decoration Decoration) *Index {
	if acquire == nil {
		acquire = func(context.Context) (func(), error) { return func() {}, nil }
	}
	index := NewMemoryIndex(nil, false)
	index.roots, index.cache, index.interval, index.defaultInterval, index.acquire = roots, cache, interval, interval, acquire
	index.lifecycle, index.decorate, index.enrich, index.enriches, index.background = ctx, decoration.Apply, decoration.Enrich, decoration.Enriches, decoration.Background
	if decoration.StartupWait <= 0 {
		_ = index.Refresh(ctx)
		return index
	}
	index.startFirstRefresh(ctx, decoration.StartupWait)
	return index
}

func (index *Index) startFirstRefresh(ctx context.Context, wait time.Duration) {
	index.err = ErrLibraryLoading
	done := make(chan struct{})
	started := time.Now()
	go func() {
		defer close(done)
		err := index.Refresh(ctx)
		index.mu.Lock()
		if err != nil && !index.ready {
			index.err = err
		}
		index.mu.Unlock()
		logFirstRefresh(err, time.Since(started))
	}()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		slog.Info("serving before the first Library scan finished", "operation", "library.startup", "outcome", "scanning", "wait_ms", wait.Milliseconds())
	}
}

func logFirstRefresh(err error, duration time.Duration) {
	switch {
	case err == nil:
		slog.Info("first Library scan published", "operation", "library.startup", "outcome", "published", "duration_ms", duration.Milliseconds())
	case errors.Is(err, context.Canceled):
		slog.Debug("first Library scan stopped", "operation", "library.startup", "outcome", "cancelled", "duration_ms", duration.Milliseconds())
	default:
		slog.Warn("first Library scan failed", "operation", "library.startup", "outcome", "failed", "duration_ms", duration.Milliseconds())
	}
}

func NewMemoryIndex(items []library.Item, ready bool) *Index {
	index := &Index{
		ready:      ready,
		reschedule: make(chan struct{}, 1), refreshes: make(chan struct{}, 1), watchChange: make(chan struct{}, 1),
		watchDebounce: libraryWatchDebounce, watchStability: libraryWatchStability, watchRetry: libraryWatchRetry, pollInterval: libraryPollInterval,
		acquire: func(context.Context) (func(), error) { return func() {}, nil }, lifecycle: context.Background(), enrichSignal: make(chan struct{}, 1),
	}
	index.watch, index.openWatcher = index.watchChanges, func() (*fsnotify.Watcher, error) { return fsnotify.NewBufferedWatcher(1024) }
	index.raw = append([]library.Item(nil), items...)
	index.current.Store(library.NewSnapshot(append([]library.Item(nil), items...), 1))
	index.resolveRoots()
	return index
}

func (index *Index) Roots() []ScanRoot {
	index.mu.RLock()
	defer index.mu.RUnlock()
	return append([]ScanRoot(nil), index.roots...)
}

func (index *Index) SetRoots(roots []ScanRoot) {
	index.mu.Lock()
	index.roots = append([]ScanRoot(nil), roots...)
	index.mu.Unlock()
	index.resolveRoots()
	index.signalWatchChange()
}

func (index *Index) UpdateRoots(ctx context.Context, roots []ScanRoot) error {
	index.SetRoots(roots)
	return index.Refresh(ctx)
}

func (index *Index) Schedule(ctx context.Context) {
	go index.runRefreshes(ctx)
	go func() {
		for index.wait(ctx) {
		}
	}()
}

func (index *Index) wait(ctx context.Context) bool {
	index.mu.RLock()
	interval := index.interval
	index.mu.RUnlock()
	return WaitRefresh(ctx, interval, index.reschedule, index.RequestRefresh)
}

func (index *Index) SetFrequency(frequency string) {
	interval := map[string]time.Duration{"off": 0, "5m": 5 * time.Minute, "15m": 15 * time.Minute, "1h": time.Hour}[frequency]
	if frequency == "default" {
		interval = index.defaultInterval
	}
	index.mu.Lock()
	index.interval = interval
	index.mu.Unlock()
	select {
	case index.reschedule <- struct{}{}:
	default:
	}
}

func (index *Index) Library() (*library.Snapshot, error) {
	index.mu.RLock()
	defer index.mu.RUnlock()
	err := index.err
	if index.ready {
		err = nil
	}
	return index.current.Load(), err
}

func (index *Index) View(policy library.Policy) (*library.View, error) {
	snapshot, err := index.Library()
	return snapshot.View(policy), err
}

func (index *Index) Snapshot() ([]library.Item, error) {
	snapshot, err := index.Library()
	return append([]library.Item(nil), snapshot.Items()...), err
}

func (index *Index) References() ([]*library.Item, error) {
	snapshot, err := index.Library()
	return snapshotReferences(snapshot), err
}

func snapshotReferences(snapshot *library.Snapshot) []*library.Item {
	items := snapshot.Items()
	references := make([]*library.Item, len(items))
	for position := range items {
		references[position] = &items[position]
	}
	return references
}

func (index *Index) Find(id string) (library.Item, bool) {
	return index.current.Load().Find(id)
}
