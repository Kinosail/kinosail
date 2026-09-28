package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	homeview "github.com/MikeO7/kinosail/packages/home"
	"github.com/MikeO7/kinosail/packages/library"
)

type homeSource struct {
	index    *libraryIndex
	progress *progressStore
	lists    *listStore
	settings *settingsStore
	updates  *updateChecker
}

func showHome(index *libraryIndex, progress *progressStore, lists *listStore, settings *settingsStore, updates *updateChecker, tmdb bool) http.HandlerFunc {
	return homeview.NewHandler(homeSource{index, progress, lists, settings, updates}, homeView, tmdb, localizedError)
}

func (source homeSource) Browse(request *http.Request) (catalog.Result, error) {
	return browseLibrary(request, source.index, source.progress, source.lists)
}

func (source homeSource) Catalog(request *http.Request, shows []library.Show, page catalog.Result, fragment bool) homeview.CatalogProjection[showCard, playlistSummary, collectionSummary] {
	projection := homeview.CatalogProjection[showCard, playlistSummary, collectionSummary]{Shows: showCards(request, source.progress, shows)}
	if fragment || page.View != "collections" && page.View != "playlists" {
		return projection
	}
	items, _ := visibleLibrary(request, source.index)
	if page.View == "collections" {
		for _, summary := range source.lists.collectionSummaries(items, page.Query) {
			if summary.Source == "default" {
				projection.DefaultCollectionCards = append(projection.DefaultCollectionCards, summary)
			} else {
				projection.CustomCollectionCards = append(projection.CustomCollectionCards, summary)
			}
		}
	}
	if page.View == "playlists" {
		projection.PlaylistCards = source.lists.PlaylistSummaries(request, items, page.Query)
	}
	return projection
}

var (
	filter      = catalog.Filter
	sortLibrary = catalog.Sort
)
