package server

import (
	"context"
	"path/filepath"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/workload"
)

func memoryLibraryIndex(items []library.Item, ready bool) *libraryIndex {
	return &libraryIndex{Index: catalog.NewMemoryIndex(items, ready)}
}

func newLibraryIndex(ctx context.Context, roots []libraryRoot, cache string, interval time.Duration, workloads *workload.Governor) *libraryIndex {
	return &libraryIndex{Index: catalog.NewIndex(ctx, roots, cache, interval, func(ctx context.Context) (func(), error) {
		return workloads.Acquire(ctx, workload.Background)
	})}
}

func sidecarTestIndex(item library.Item) *libraryIndex {
	index := memoryLibraryIndex([]library.Item{item}, true)
	index.SetRoots([]libraryRoot{{Path: filepath.Dir(item.Path)}})
	return index
}
