package catalog

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type ScanRoot struct {
	Path, Namespace string
}

func ScanRoots[T any](values []T, fields func(T) (string, string)) []ScanRoot {
	roots := make([]ScanRoot, len(values))
	for position, value := range values {
		roots[position].Path, roots[position].Namespace = fields(value)
	}
	return roots
}

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

func Scan(ctx context.Context, roots []ScanRoot, decorate func([]library.Item) []library.Item) ([]library.Item, error) {
	scanners := make([]*library.Scanner, len(roots))
	for position, root := range roots {
		scanners[position] = library.NewScanner(root.Path, root.Namespace)
	}
	items, err := scanRoots(ctx, scanners, func(scanner *library.Scanner) ([]library.Item, error) { return scanner.Scan(ctx) })
	if err != nil {
		return items, err
	}
	if decorate != nil {
		items = decorate(items)
	}
	return items, nil
}

func scanRoots(ctx context.Context, scanners []*library.Scanner, scan func(*library.Scanner) ([]library.Item, error)) ([]library.Item, error) {
	items := make([]library.Item, 0)
	if len(scanners) == 0 {
		return items, nil
	}
	type result struct {
		items []library.Item
		err   error
	}
	results := make([]result, len(scanners))
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(len(scanners), max(1, min(2, runtime.GOMAXPROCS(0)-1))) {
		workers.Go(func() {
			for position := range jobs {
				results[position].items, results[position].err = scan(scanners[position])
			}
		})
	}
dispatch:
	for position := range scanners {
		select {
		case jobs <- position:
		case <-ctx.Done():
			break dispatch
		}
	}
	close(jobs)
	workers.Wait()
	for _, result := range results {
		items = append(items, result.items...)
		if result.err != nil {
			return items, result.err
		}
	}
	return items, ctx.Err()
}

func (index *Index) Refresh(ctx context.Context) error {
	index.resolveRoots()
	return RefreshWork(ctx, &index.refreshMu, index.acquire, func(ctx context.Context) error {
		scanners := index.rootScanners()
		raw, err := scanRoots(ctx, scanners, func(scanner *library.Scanner) ([]library.Item, error) { return scanner.Scan(ctx) })
		return index.commit(ctx, raw, err, true)
	})
}

func (index *Index) RefreshPaths(ctx context.Context, paths []string) error {
	return index.refreshChanged(ctx, paths)
}

func (index *Index) refreshChanged(ctx context.Context, paths []string) error {
	return RefreshWork(ctx, &index.refreshMu, index.acquire, func(ctx context.Context) error {
		scanners := index.rootScanners()
		raw, err := scanRoots(ctx, scanners, func(scanner *library.Scanner) ([]library.Item, error) {
			return scanner.Rescan(ctx, pathsWithin(scanner.Root(), paths))
		})
		return index.commit(ctx, raw, err, false)
	})
}

func (index *Index) rootScanners() []*library.Scanner {
	roots := index.Roots()
	scanners := make([]*library.Scanner, len(roots))
	next := make(map[ScanRoot]*library.Scanner, len(roots))
	for position, root := range roots {
		scanner := index.scanners[root]
		if scanner == nil {
			scanner = library.NewScanner(root.Path, root.Namespace)
		}
		scanners[position], next[root] = scanner, scanner
	}
	index.scanners = next
	return scanners
}

func pathsWithin(root string, paths []string) []string {
	within := make([]string, 0)
	prefix := root + string(filepath.Separator)
	for _, path := range paths {
		if path == root || strings.HasPrefix(path, prefix) {
			within = append(within, path)
		}
	}
	return within
}
