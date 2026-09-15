package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
)

func browseLibrary(request *http.Request, index *libraryIndex, progress *progressStore, lists *listStore) (catalog.Result, error) {
	return catalog.BrowseLibrary(request.URL.Query(), preferredLanguage(request), index.References, func() catalog.BrowseAccess {
		viewer := currentViewer(request)
		return progress.BrowseAccess(&lists.mu, &lists.values, request, viewerPolicy(viewer).Allows)
	})
}
