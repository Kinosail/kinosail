package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
)

var (
	detailSources = catalog.MustDetailSources(catalog.DetailPresentation{
		Product: "Kinosail Player", ThemeVersion: "cinema-1", StyleVersion: "electric-29",
	})
	detailPages = catalog.NewDetailHTTP(
		newLocalizedTemplate("album", detailSources.Album),
		newLocalizedTemplate("book", detailSources.Book),
		func(request *http.Request) catalog.DetailViewer {
			viewer := currentViewer(request)
			return catalog.DetailViewer{Owner: viewer.Owner, Downloads: viewer.Downloads}
		},
		localizedError,
		localizedNotFound,
	)
)

func browseAlbum(index *libraryIndex) http.HandlerFunc {
	return detailPages.Album(index)
}
