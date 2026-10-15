package catalog

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/fsnotify/fsnotify"
)

type Index struct {
	mu                sync.RWMutex
	refreshMu         sync.Mutex
	roots             []ScanRoot
	cache             string
	items             []library.Item
	titleOrders       map[titleOrderKey]*titleOrder
	titleOrderUse     uint64
	titleOrderVersion uint64
	byID              map[string]int
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
	snapshot          func(map[string]struct{}) (map[string]fileStamp, error)
}

func NewIndex(ctx context.Context, roots []ScanRoot, cache string, interval time.Duration, acquire func(context.Context) (func(), error)) *Index {
	if acquire == nil {
		acquire = func(context.Context) (func(), error) { return func() {}, nil }
	}
	index := NewMemoryIndex(nil, false)
	index.roots, index.cache, index.interval, index.defaultInterval, index.acquire = roots, cache, interval, interval, acquire
	_ = index.Refresh(ctx)
	return index
}

func NewMemoryIndex(items []library.Item, ready bool) *Index {
	copyItems := append([]library.Item(nil), items...)
	index := &Index{
		items: copyItems, byID: ItemsByID(copyItems), ready: ready,
		reschedule: make(chan struct{}, 1), refreshes: make(chan struct{}, 1), watchChange: make(chan struct{}, 1),
		watchDebounce: libraryWatchDebounce, watchStability: libraryWatchStability, watchRetry: libraryWatchRetry, pollInterval: libraryPollInterval,
		acquire: func(context.Context) (func(), error) { return func() {}, nil },
	}
	index.watch, index.openWatcher, index.snapshot = index.watchChanges, func() (*fsnotify.Watcher, error) { return fsnotify.NewBufferedWatcher(1024) }, snapshotFiles
	return index
}

func (index *Index) Safe(path string) bool {
	index.mu.RLock()
	roots, cache := append([]ScanRoot(nil), index.roots...), index.cache
	index.mu.RUnlock()
	allowed := make([]string, 1, len(roots)+1)
	allowed[0] = cache
	for _, root := range roots {
		allowed = append(allowed, root.Path)
	}
	return SafePath(path, allowed)
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
	index.signalWatchChange()
}

func (index *Index) UpdateRoots(ctx context.Context, roots []ScanRoot) error {
	index.SetRoots(roots)
	return index.Refresh(ctx)
}

func (index *Index) Refresh(ctx context.Context) error {
	return RefreshIndex(ctx, &index.refreshMu, &index.mu, &index.roots, index.acquire, index.storage(), func(root ScanRoot) (string, string) {
		return root.Path, root.Namespace
	})
}

func (index *Index) SetDecorator(decorate func([]library.Item) []library.Item) {
	index.storage().SetDecorator(decorate)
}

func (index *Index) AddAnalyzer(analyze func([]library.Item)) {
	index.storage().AddAnalyzer(analyze)
}

func (index *Index) Status() (int, time.Time, error) {
	return index.storage().Status()
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

func (index *Index) runRefreshes(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-index.refreshes:
			if err := index.Refresh(ctx); err != nil {
				slog.Warn("background Library scan failed", "error", err)
			} else {
				items, scanned, _ := index.Status()
				slog.Debug("background Library scan completed", "items", items, "scanned", scanned)
			}
		}
	}
}

func (index *Index) RequestRefresh() {
	select {
	case index.refreshes <- struct{}{}:
	default:
	}
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

func (index *Index) Snapshot() ([]library.Item, error) {
	index.mu.RLock()
	defer index.mu.RUnlock()
	err := index.err
	if index.ready {
		err = nil
	}
	return append([]library.Item(nil), index.items...), err
}

func (index *Index) References() ([]*library.Item, error) {
	index.mu.RLock()
	defer index.mu.RUnlock()
	err := index.err
	if index.ready {
		err = nil
	}
	items := make([]*library.Item, len(index.items))
	for position := range index.items {
		items[position] = &index.items[position]
	}
	return items, err
}

func (index *Index) Find(id string) (library.Item, bool) {
	index.mu.RLock()
	defer index.mu.RUnlock()
	position, found := index.byID[id]
	if !found {
		return library.Item{}, false
	}
	return index.items[position], true
}

func (index *Index) storage() IndexStorage {
	return IndexStorage{Mutex: &index.mu, Items: &index.items, ByID: &index.byID, Err: &index.err, Ready: &index.ready, Scanned: &index.scanned, Decorator: &index.decorate, Analyzers: &index.analyzers, published: func() { index.titleOrders = nil; index.titleOrderVersion++ }}
}
