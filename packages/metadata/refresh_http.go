package metadata

import (
	"context"
	"net/http"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/library"
)

// RefreshHandler binds Player's metadata refresh flow to the app's localized errors.
func RefreshHandler(index BulkIndex, fetch func(context.Context, library.Item) error, fail BulkFailure, notFound http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if !httpguard.EmptyMutationRequest(writer, request) {
			fail(writer, request, "metadata request is invalid", http.StatusBadRequest)
			return
		}
		item, found := index.Find(request.PathValue("id"))
		if !found || item.Kind != "video" {
			notFound(writer, request)
			return
		}
		if err := fetch(request.Context(), item); err != nil || index.Refresh(request.Context()) != nil {
			fail(writer, request, "metadata provider unavailable", http.StatusBadGateway)
			return
		}
		http.Redirect(writer, request, "/watch/"+item.ID, http.StatusSeeOther)
	}
}
