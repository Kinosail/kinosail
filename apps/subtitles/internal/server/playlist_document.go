package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/catalogapi"
)

func newListHandlers(index *libraryIndex, lists *listStore) catalog.ListHandlers {
	return catalogapi.NewListHandlers(index, lists, func(request *http.Request) string { return currentViewer(request).ID }, localizedError, localizedNotFound)
}
