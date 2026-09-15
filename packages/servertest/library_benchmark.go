package servertest

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

// LibraryBenchmark binds the shared index and search implementations.
type LibraryBenchmark struct {
	Index  func([]library.Item) func(string) (library.Item, bool)
	Search func([]library.Item, string) []library.Item
}

// FindDoesNotCopy verifies the allocation-free index lookup contract.
func (fixture LibraryBenchmark) FindDoesNotCopy(t *testing.T) {
	find := fixture.index(10_000)
	if allocations := testing.AllocsPerRun(100, func() {
		if item, found := find("9999"); !found || item.ID != "9999" {
			t.Fatal("last Library item was not found")
		}
	}); allocations != 0 {
		t.Fatalf("find allocations = %v, want 0", allocations)
	}
}

// BenchmarkFind measures index lookup without copying the library.
func (fixture LibraryBenchmark) BenchmarkFind(b *testing.B) {
	find := fixture.index(10_000)
	b.ReportAllocs()
	for b.Loop() {
		if _, found := find("9999"); !found {
			b.Fatal("last Library item was not found")
		}
	}
}

// BenchmarkSearch10K measures a selective search across a large library.
func (fixture LibraryBenchmark) BenchmarkSearch10K(b *testing.B) {
	items := make([]library.Item, 10_000)
	for position := range items {
		items[position] = library.Item{ID: strconv.Itoa(position), Title: fmt.Sprintf("Library title %05d", position), Plot: "A household media item with searchable metadata.", Genres: "Drama"}
	}
	items[len(items)-1].Title = "Unique lighthouse title"
	b.ReportAllocs()
	for b.Loop() {
		if matches := fixture.Search(items, "unique lighthouse"); len(matches) != 1 {
			b.Fatalf("matches = %d", len(matches))
		}
	}
}

func (fixture LibraryBenchmark) index(count int) func(string) (library.Item, bool) {
	items := make([]library.Item, count)
	for position := range items {
		items[position].ID = strconv.Itoa(position)
	}
	return fixture.Index(items)
}
