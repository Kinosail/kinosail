package catalog

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

// SafePath reports whether a resolved path stays inside one configured root.
func SafePath(path string, allowed []string) bool {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	for _, candidate := range allowed {
		root, rootErr := filepath.EvalSymlinks(candidate)
		relative, relErr := filepath.Rel(root, resolved)
		if candidate != "" && rootErr == nil && relErr == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// ScanRoot is one Library scan path and its stable namespace.
type ScanRoot struct {
	Path, Namespace string
}

// ScanRoots maps app-owned root records into the canonical scan contract.
func ScanRoots[T any](values []T, fields func(T) (string, string)) []ScanRoot {
	roots := make([]ScanRoot, len(values))
	for position, value := range values {
		roots[position].Path, roots[position].Namespace = fields(value)
	}
	return roots
}

// IndexStorage connects shared refresh rules to an app-owned watcher index.
type IndexStorage struct {
	Mutex     *sync.RWMutex
	Items     *[]library.Item
	ByID      *map[string]int
	Err       *error
	Ready     *bool
	Scanned   *time.Time
	Decorator *func([]library.Item) []library.Item
	Analyzers *[]func([]library.Item)
}

// RefreshWork serializes a bounded background-work lease around one index refresh.
func RefreshWork(ctx context.Context, mutex *sync.Mutex, acquire func(context.Context) (func(), error), refresh func(context.Context) error) error {
	mutex.Lock()
	defer mutex.Unlock()
	release, err := acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	return refresh(ctx)
}

// RefreshIndex snapshots app-owned roots and publishes their scan under one work lease.
func RefreshIndex[T any](ctx context.Context, refreshMutex *sync.Mutex, rootMutex *sync.RWMutex, roots *[]T, acquire func(context.Context) (func(), error), storage IndexStorage, fields func(T) (string, string)) error {
	return RefreshWork(ctx, refreshMutex, acquire, func(ctx context.Context) error {
		rootMutex.RLock()
		values := append([]T(nil), (*roots)...)
		rootMutex.RUnlock()
		return storage.Refresh(ctx, ScanRoots(values, fields))
	})
}

// WaitRefresh waits for cancellation, rescheduling, or the next refresh interval.
func WaitRefresh(ctx context.Context, interval time.Duration, reschedule <-chan struct{}, request func()) bool {
	if interval <= 0 {
		select {
		case <-ctx.Done():
			return false
		case <-reschedule:
			return true
		}
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-reschedule:
		return true
	case <-timer.C:
		request()
		return true
	}
}

// Scan reads all roots and applies one complete decoration pipeline.
func Scan(ctx context.Context, roots []ScanRoot, decorate func([]library.Item) []library.Item) ([]library.Item, error) {
	items := make([]library.Item, 0)
	for _, root := range roots {
		scanned, err := library.ScanContext(ctx, root.Path, root.Namespace)
		items = append(items, scanned...)
		if err != nil {
			return items, err
		}
	}
	if decorate != nil {
		items = decorate(items)
	}
	return items, nil
}

// ItemsByID indexes the first occurrence of each stable Library ID.
func ItemsByID(items []library.Item) map[string]int {
	byID := make(map[string]int, len(items))
	for position, item := range items {
		if _, found := byID[item.ID]; !found {
			byID[item.ID] = position
		}
	}
	return byID
}

// Refresh scans and atomically publishes one complete decorated index.
func (storage IndexStorage) Refresh(ctx context.Context, roots []ScanRoot) error {
	storage.Mutex.RLock()
	decorate := *storage.Decorator
	storage.Mutex.RUnlock()
	items, err := Scan(ctx, roots, decorate)
	storage.Mutex.Lock()
	*storage.Err = err
	if err == nil {
		*storage.Items, *storage.ByID, *storage.Ready, *storage.Scanned = items, ItemsByID(items), true, time.Now()
	}
	analyzers := append([]func([]library.Item){}, (*storage.Analyzers)...)
	storage.Mutex.Unlock()
	if err == nil {
		for _, analyze := range analyzers {
			analyze(items)
		}
	}
	return err
}

// SetDecorator replaces the index projection and rebuilds its ID lookup.
func (storage IndexStorage) SetDecorator(decorate func([]library.Item) []library.Item) {
	storage.Mutex.Lock()
	*storage.Decorator = decorate
	*storage.Items = decorate(*storage.Items)
	*storage.ByID = ItemsByID(*storage.Items)
	storage.Mutex.Unlock()
}

// AddAnalyzer registers an analyzer and sends it a detached current snapshot.
func (storage IndexStorage) AddAnalyzer(analyze func([]library.Item)) {
	storage.Mutex.Lock()
	*storage.Analyzers = append(*storage.Analyzers, analyze)
	items := append([]library.Item(nil), (*storage.Items)...)
	storage.Mutex.Unlock()
	analyze(items)
}

// Status returns the current item count, scan time, and scan error.
func (storage IndexStorage) Status() (int, time.Time, error) {
	storage.Mutex.RLock()
	defer storage.Mutex.RUnlock()
	return len(*storage.Items), *storage.Scanned, *storage.Err
}
