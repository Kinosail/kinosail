package operations

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/metadata"
)

func TestRefreshMetadataBoundsConcurrencyAndStopsOnCancellation(t *testing.T) { //nolint:cyclop,funlen // Cancellation must stop launch while active provider work drains.
	items := make([]library.Item, 8)
	for position := range items {
		items[position] = library.Item{ID: string(rune('a' + position)), Kind: "video"}
	}
	ctx, cancel := context.WithCancel(t.Context())
	started, release := make(chan struct{}, metadataWorkers), make(chan struct{})
	var active, maximum, resolves atomic.Int32
	config := metadataFixture(items, nil)
	config.Resolve = func(context.Context, library.Item) (metadata.Result, error) {
		resolves.Add(1)
		current := active.Add(1)
		for observed := maximum.Load(); current > observed && !maximum.CompareAndSwap(observed, current); observed = maximum.Load() {
		}
		started <- struct{}{}
		<-release
		active.Add(-1)
		return metadata.Result{Record: metadata.Record{Title: "resolved"}}, nil
	}
	result := make(chan error, 1)
	go func() { result <- RefreshMetadata(ctx, config) }()
	for range metadataWorkers {
		select {
		case <-started:
		case err := <-result:
			t.Fatalf("refresh stopped before worker limit: %v", err)
		case <-time.After(time.Second):
			t.Fatal("refresh did not start bounded workers")
		}
	}
	cancel()
	close(release)
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) || maximum.Load() != metadataWorkers || resolves.Load() != metadataWorkers {
			t.Fatalf("error=%v maximum=%d resolves=%d", err, maximum.Load(), resolves.Load())
		}
	case <-time.After(time.Second):
		t.Fatal("canceled refresh did not stop")
	}
}

func TestRefreshMetadataNeverExceedsWorkerLimit(t *testing.T) {
	t.Parallel()
	items := make([]library.Item, 12)
	for position := range items {
		items[position] = library.Item{ID: string(rune('a' + position)), Kind: "video"}
	}
	var active, maximum atomic.Int32
	config := metadataFixture(items, nil)
	config.Resolve = func(_ context.Context, item library.Item) (metadata.Result, error) {
		current := active.Add(1)
		for observed := maximum.Load(); current > observed && !maximum.CompareAndSwap(observed, current); observed = maximum.Load() {
		}
		time.Sleep(2 * time.Millisecond)
		active.Add(-1)
		return metadata.Result{Record: metadata.Record{Title: item.ID}}, nil
	}
	if err := RefreshMetadata(t.Context(), config); err != nil || maximum.Load() < 2 || maximum.Load() > metadataWorkers {
		t.Fatalf("error=%v maximum=%d", err, maximum.Load())
	}
}
