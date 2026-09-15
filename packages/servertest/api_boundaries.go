package servertest

import (
	"net/http"
	"testing"
)

// VersionedAPIMissingResources verifies missing-resource failures across public capabilities.
func (fixture LibraryAPIFixture) VersionedAPIMissingResources(t *testing.T) {
	handler, token := fixture.Server(t)
	for _, call := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/items/missing"},
		{http.MethodPut, "/api/v1/items/missing/progress"},
		{http.MethodDelete, "/api/v1/items/missing/continue-watching"},
		{http.MethodGet, "/api/v1/audio/missing/queue"},
		{http.MethodGet, "/api/v1/books/missing/reader"},
		{http.MethodGet, "/api/v1/shows/missing"},
		{http.MethodGet, "/api/v1/albums/missing"},
		{http.MethodGet, "/api/v1/playlists/Missing"},
		{http.MethodGet, "/api/v1/collections/Missing"},
		{http.MethodGet, "/api/v1/items/missing/playback"},
		{http.MethodPost, "/api/v1/items/missing/playback-events"},
		{http.MethodPost, "/api/v1/items/missing/metadata/refresh"},
		{http.MethodDelete, "/api/v1/downloads/missing"},
		{http.MethodGet, "/api/v1/downloads/missing"},
		{http.MethodGet, "/api/v1/downloads/missing/file"},
		{http.MethodDelete, "/api/v1/profiles/missing"},
		{http.MethodDelete, "/api/v1/devices/missing"},
		{http.MethodDelete, "/api/v1/api-keys/missing"},
		{http.MethodPost, "/api/v1/viewing-imports/missing/apply"},
		{http.MethodPost, "/api/v1/viewing-syncs/missing/run"},
		{http.MethodDelete, "/api/v1/viewing-syncs/missing"},
	} {
		response := APICall(t, handler, token, call.method, call.path, nil)
		if response.Code < http.StatusBadRequest {
			t.Fatalf("%s %s = %d %q", call.method, call.path, response.Code, response.Body.String())
		}
	}
}
