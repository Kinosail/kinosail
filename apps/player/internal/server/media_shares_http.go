package server

import (
	"html/template"
	"net/http"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/mediashares"
)

var (
	mediaShareViews     = mediashares.NewViews("Player", "80", func(name, source string) *template.Template { return httpguard.NewCSRFTemplate(name, source, uiIcon) })
	mediaShareItemsView = mediaShareViews.Items
)

func registerMediaShares(mux *http.ServeMux, store *mediaShareStore, auth *authentication) {
	mediashares.Register(mux, store, mediaShareViews, auth.owner, serveScript, httpguard.ExecuteCSRFTemplate, localizedError)
}
