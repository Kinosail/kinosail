package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/servertest"
)

var libraryBenchmark = servertest.LibraryBenchmark{
	Index: func(items []library.Item) func(string) (library.Item, bool) {
		return memoryLibraryIndex(items, true).Find
	},
	Search: filter,
}

func TestLibraryFindDoesNotCopyTheLibrary(t *testing.T) { libraryBenchmark.FindDoesNotCopy(t) }
func BenchmarkLibraryFind(b *testing.B)                 { libraryBenchmark.BenchmarkFind(b) }
func BenchmarkLibrarySearch10K(b *testing.B)            { libraryBenchmark.BenchmarkSearch10K(b) }
