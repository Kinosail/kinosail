package catalog

import (
	"context"
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
	index := &Index{
		ready:      ready,
		reschedule: make(chan struct{}, 1), refreshes: make(chan struct{}, 1), watchChange: make(chan struct{}, 1),
		watchDebounce: libraryWatchDebounce, watchStability: libraryWatchStability, watchRetry: libraryWatchRetry, pollInterval: libraryPollInterval,
		acquire: func(context.Context) (func(), error) { return func() {}, nil },
	}
	index.watch, index.openWatcher, index.snapshot = index.watchChanges, func() (*fsnotify.Watcher, error) { return fsnotify.NewBufferedWatcher(1024) }, snapshotFiles
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

func (index *Index) Refresh(ctx context.Context) error {
	index.resolveRoots()
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

func (index *Index) storage() IndexStorage {
	return IndexStorage{Mutex: &index.mu, Library: &index.current, Err: &index.err, Ready: &index.ready, Scanned: &index.scanned, Decorator: &index.decorate, Analyzers: &index.analyzers, published: func() { index.titleOrders = nil; index.titleOrderVersion++ }}
}
