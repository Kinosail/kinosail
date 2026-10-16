package server

import (
	_ "embed"
	"net/http"

	"github.com/MikeO7/kinosail/packages/auditjournal"
	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/productapi"
)

//go:embed api_openapi.json
var openAPISpec []byte

func registerProductAPI(mux *http.ServeMux, api apiServices) {
	owner := func(pattern string, handler http.Handler) { mux.Handle(pattern, api.auth.owner(handler)) }
	mux.HandleFunc("GET /api/v1", apiRoot)
	mux.HandleFunc("GET /api/v1/me", productapi.Me(func(request *http.Request) productapi.MeState {
		return productapi.MeState{Server: api.settings.serverName(), Viewer: currentViewer(request), SSO: api.auth.sso, Language: preferredLanguage(request), LanguagePreference: languagePreference(request), Languages: languageOptions()}
	}))
	mux.HandleFunc("PUT /api/v1/me/language", apiLanguage)
	mux.HandleFunc("GET /api/v1/openapi.json", serveOpenAPI)
	owner("GET /api/v1/activity", auditjournal.ActivityAPI(api.auth.audit.HTTPTracker, func() []catalog.PlaybackView { return nil }, writeJSON))
	owner("GET /api/v1/activity/export", auditjournal.ExportAPI(api.auth.audit.HTTPTracker))
	mux.Handle("GET /api/v1/backup", api.auth.freshOwner(downloadBackup(api.settings)))
}

func apiRoot(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, map[string]any{"name": "Kinosail API", "version": "v1", "openapi": "/api/v1/openapi.json", "libraryContent": "direct-only"}, http.StatusOK)
}

func serveOpenAPI(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/vnd.oai.openapi+json")
	writer.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = writer.Write(openAPISpec)
}

type apiContractError string

func (err apiContractError) Error() string { return string(err) }
