package server

import (
	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

type collectionSummary = catalog.CollectionSummary

func (store *listStore) collectionSummaries(items []library.Item, query string) []collectionSummary {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.storage().CollectionSummaries(items, query)
}

func (store *listStore) CollectionSummaries(items []library.Item) any {
	return store.collectionSummaries(items, "")
}

var collectionMatches = catalog.CollectionMatches
