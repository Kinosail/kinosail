package home

import (
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func BenchmarkRecentlyAddedLargeLibrary(b *testing.B) {
	items := make([]library.Item, 10_000)
	for index := range items {
		items[index] = library.Item{Kind: "video", Title: "Movie", Added: time.Unix(int64(index), 0)}
	}
	b.ReportAllocs()
	for b.Loop() {
		if cards := RecentlyAdded(items, nil, nil); len(cards) != recentItemLimit {
			b.Fatalf("recent cards = %d", len(cards))
		}
	}
}
