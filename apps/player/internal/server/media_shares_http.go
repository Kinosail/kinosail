package server

import (
	"html/template"
	"net/http"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/localization"
	"github.com/MikeO7/kinosail/packages/mediashares"
)

var (
	localizedMediaShareViews = map[*template.Template]*localization.TemplateSet{}
	mediaShareViews          = mediashares.NewViews("Player", "80", newMediaShareTemplate)
	mediaShareItemsView      = mediaShareViews.Items
)

func newMediaShareTemplate(name, source string) *template.Template {
	view := newCSRFTemplate(name, source)
	localizedMediaShareViews[view] = newLocalizedTemplate(name, source)
	return view
}

func executeMediaShareTemplate(view *template.Template, writer http.ResponseWriter, request *http.Request, data any) error {
	if localized := localizedMediaShareViews[view]; localized != nil {
		return localized.Execute(writer, request, data)
	}
	return httpguard.ExecuteCSRFTemplate(view, writer, request, data)
}

func registerMediaShares(mux *http.ServeMux, store *mediaShareStore, auth *authentication) {
	mediashares.Register(mux, store, mediaShareViews, auth.owner, serveScript, executeMediaShareTemplate, localizedError)
}
