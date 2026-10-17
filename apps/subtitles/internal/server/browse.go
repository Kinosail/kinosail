package server

import (
	"net/http"
	"sync"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/catalogapi"
	"github.com/MikeO7/kinosail/packages/library"
)

var (
	browseProgressMu sync.Mutex
	browseListMu     sync.RWMutex
	browseProgress   = map[string]catalog.PlaybackState{}
	browseListed     = map[string]bool{}
)

func browseLibrary(request *http.Request, index *libraryIndex) (catalog.Result, error) {
	return index.BrowseLibrary(request.Context(), request.URL.Query(), preferredLanguage(request), func() catalog.BrowseAccess {
		viewer := currentViewer(request)
		policy := viewerPolicy(viewer)
		return catalog.NewBrowseAccess(&browseProgressMu, &browseListMu, &browseProgress, &browseListed, viewer.ID, viewer.Owner, policy.Allows).WithPolicy(policy)
	})
}

func libraryClientItem(request *http.Request, item library.Item) any {
	return catalogapi.ProjectViewerItem(item, catalog.PlaybackState{}, currentViewer(request))
}
