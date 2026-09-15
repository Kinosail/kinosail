package server

import (
	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func memoryLibraryIndex(items []library.Item, ready bool) *libraryIndex {
	return &libraryIndex{Index: catalog.NewMemoryIndex(items, ready)}
}
