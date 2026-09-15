package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/library"
)

func visibleLibrary(request *http.Request, index *libraryIndex) ([]library.Item, error) {
	return identitycore.VisibleLibrary(index, currentViewer(request))
}

func visibleItem(request *http.Request, index *libraryIndex, id string) (library.Item, bool) {
	return identitycore.VisibleItem(index, currentViewer(request), id)
}

var (
	viewerPolicy = identitycore.LibraryPolicy
	canView      = identitycore.CanView
)
