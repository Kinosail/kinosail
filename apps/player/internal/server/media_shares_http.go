package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/mediashares"
)

var (
	mediaShareViews     = mediashares.NewViews("Player", "80", newCSRFTemplate)
	mediaShareItemsView = mediaShareViews.Items
)

func registerMediaShares(mux *http.ServeMux, store *mediaShareStore, auth *authentication) {
	mediashares.Register(mux, store, mediaShareViews, auth.owner, serveScript, executeCSRFTemplate, localizedError)
}
