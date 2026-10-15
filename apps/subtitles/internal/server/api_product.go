package server

import (
	"context"
	_ "embed"
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/auditjournal"
	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
	sharedmetadata "github.com/MikeO7/kinosail/packages/metadata"
	"github.com/MikeO7/kinosail/packages/productapi"
	"github.com/MikeO7/kinosail/packages/remoteaccess"
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
	owner("POST /api/v1/metadata/bulk", productapi.BulkEditMetadata(productapi.BulkMetadataFunc(func(ctx context.Context, ids []string, patch productapi.MetadataPatch) (int, error) {
		updates, err := api.metadata.applyPatch(ctx, api.index, ids, metadataPatch(patch))
		return len(updates), err
	}), errMetadataSelection, errMetadataFields))
	metadata := productMetadata{index: api.index, store: api.metadata}
	owner("PUT /api/v1/items/{id}/metadata", productapi.EditMetadata(metadata, metadata))
	owner("POST /api/v1/items/{id}/metadata/refresh", productapi.RefreshMetadata(metadata, metadata))
	owner("GET /api/v1/remote-access", remoteaccess.StatusAPI(api.internet, func(status remoteaccess.Status) remoteReadiness {
		return securePublicReadiness(status, api.auth.profiles.list(), api.auth.profiles.err, time.Now())
	}, writeJSON))
	contractError := func(message string) error { return apiContractError(message) }
	owner("POST /api/v1/remote-access/kill", remoteaccess.KillAPI(api.internet, func() error {
		return revokePublicAuthorization(api.auth.profiles, api.auth.passkeys)
	}, contractError, apiError))
	owner("DELETE /api/v1/remote-access/kill", remoteaccess.ResetKillAPI(api.internet, contractError, apiError))
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

type productMetadata struct {
	index *libraryIndex
	store *metadataStore
}

func (service productMetadata) Find(id string) (library.Item, bool) { return service.index.Find(id) }

func (service productMetadata) Refresh(ctx context.Context) error { return service.index.Refresh(ctx) }

func (service productMetadata) Record(id string) sharedmetadata.Record {
	service.store.mu.RLock()
	defer service.store.mu.RUnlock()
	return service.store.records[id]
}

func (service productMetadata) Save(id string, record sharedmetadata.Record) error {
	return service.store.set(id, record)
}

func (service productMetadata) Fetch(ctx context.Context, item library.Item) error {
	return service.store.fetch(ctx, item)
}
