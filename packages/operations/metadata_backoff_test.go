package operations

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/metadata"
)

func TestRefreshMetadataBacksOffProviderMissesPerTitle(t *testing.T) {
	items := []library.Item{
		{ID: "unknown", Kind: "video", Title: "Unknown Film"},
		{ID: "episode", Kind: "video", Library: "TV", Show: "Unknown Show", Title: "Pilot"},
	}
	var resolves atomic.Int32
	config := metadataFixture(items, map[string]metadata.Record{})
	config.Misses = metadata.NewMisses()
	config.Resolve = func(context.Context, library.Item) (metadata.Result, error) {
		resolves.Add(1)
		return metadata.Result{}, errors.New("metadata was not found")
	}
	if err := RefreshMetadata(t.Context(), config); err == nil {
		t.Fatal("provider misses were reported as success")
	}
	for range 2 {
		_ = RefreshMetadata(t.Context(), config)
	}
	if resolves.Load() != 2 {
		t.Fatalf("provider lookups after repeated misses = %d, want 2", resolves.Load())
	}
	config.Items = []library.Item{{ID: "unknown", Kind: "video", Title: "Renamed Film"}, items[1]}
	_ = RefreshMetadata(t.Context(), config)
	if resolves.Load() != 3 {
		t.Fatalf("renamed title did not retry: lookups = %d", resolves.Load())
	}
}
