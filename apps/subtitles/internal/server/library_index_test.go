package server

import (
	"context"
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
