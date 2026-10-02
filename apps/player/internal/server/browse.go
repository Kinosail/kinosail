package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
)

func browseLibrary(request *http.Request, index *libraryIndex, progress *progressStore, lists *listStore) (catalog.Result, error) {
	return index.BrowseLibrary(request.Context(), request.URL.Query(), preferredLanguage(request), func() catalog.BrowseAccess {
		viewer := currentViewer(request)
		return progress.BrowseAccess(&lists.mu, &lists.values, request, viewerPolicy(viewer).Allows)
	})
}
