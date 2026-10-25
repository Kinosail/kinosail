package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/downloads"
	"github.com/MikeO7/kinosail/packages/library"
)

func (manager *downloadManager) registerAPI(mux *http.ServeMux, index *libraryIndex) {
	downloads.RegisterAPI(mux, manager.Manager, downloadAccess(index))
}

func downloadAccess(index *libraryIndex) downloads.Access {
	return downloads.NewAccess(currentViewer, func(request *http.Request, id string) (library.Item, bool) {
		return visibleItem(request, index, id)
	})
}
