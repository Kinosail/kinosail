package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

type playlistSummary = catalog.PlaylistSummary

func (store *listStore) PlaylistSummaries(request *http.Request, items []library.Item, query string) []playlistSummary {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.storage().PlaylistSummaries(currentViewer(request).ID, items, query)
}

func (store *listStore) PlaylistSummariesJSON(request *http.Request, items []library.Item, query string) any {
	return store.PlaylistSummaries(request, items, query)
}

func (store *listStore) playlistRule(request *http.Request, name string) (playlistRule, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.storage().PlaylistRule(currentViewer(request).ID, name)
}

var describePlaylistRule = catalog.DescribePlaylistRule
