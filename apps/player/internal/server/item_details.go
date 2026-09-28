package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func showItemDetails(index *libraryIndex) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RawQuery != "" {
			localizedError(writer, request, "invalid title request", http.StatusBadRequest)
			return
		}
		item, found := visibleItem(request, index, request.PathValue("id"))
		if !found || item.Kind != "video" {
			localizedNotFound(writer, request)
			return
		}
		http.Redirect(writer, request, "/watch/"+item.ID, http.StatusFound)
	}
}

func saveDetailsList(index *libraryIndex, lists *listStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		action := catalog.SaveList(request, currentViewer(request).ID, func(id string) (library.Item, bool) {
			item, found := visibleItem(request, index, id)
			return item, found && item.Kind == "video"
		}, lists.SetListed)
		action.Serve(writer, request, localizedError, localizedNotFound)
	}
}
