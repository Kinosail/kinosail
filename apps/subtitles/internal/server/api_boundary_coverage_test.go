package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVersionedAPIRejectsMalformedMutationBodies(t *testing.T) { //nolint:funlen // The table keeps the JSON contract identical across every public mutation seam.
	handler, token := apiServer(t)
	itemID := firstAPIItemID(t, handler, token)
	paths := []struct{ method, path string }{
		{http.MethodPut, "/api/v1/me/language"},
		{http.MethodPut, "/api/v1/items/" + itemID + "/progress"},
		{http.MethodPost, "/api/v1/items/" + itemID + "/playback-events"},
		{http.MethodPut, "/api/v1/items/" + itemID + "/list"},
		{http.MethodPost, "/api/v1/items/" + itemID + "/downloads"},
		{http.MethodPost, "/api/v1/playlists"},
		{http.MethodPost, "/api/v1/smart-playlists"},
		{http.MethodPut, "/api/v1/playlists/Missing/items/" + itemID},
		{http.MethodPut, "/api/v1/playlists/Missing/order"},
		{http.MethodPost, "/api/v1/collections"},
		{http.MethodPut, "/api/v1/collections/Missing/items/" + itemID},
		{http.MethodPut, "/api/v1/items/" + itemID + "/metadata"},
		{http.MethodPost, "/api/v1/items/" + itemID + "/subtitles"},
		{http.MethodPut, "/api/v1/items/" + itemID + "/markers"},
		{http.MethodPut, "/api/v1/configuration/server.name"},
		{http.MethodPut, "/api/v1/settings/server"},
		{http.MethodPut, "/api/v1/settings/navigation"},
		{http.MethodPut, "/api/v1/settings/onboarding"},
		{http.MethodPut, "/api/v1/settings/updates"},
		{http.MethodPut, "/api/v1/settings/mfa"},
		{http.MethodPut, "/api/v1/settings/session-timeouts"},
		{http.MethodPut, "/api/v1/settings/playback"},
		{http.MethodPut, "/api/v1/settings/transcoder"},
		{http.MethodPut, "/api/v1/settings/subtitles"},
		{http.MethodPut, "/api/v1/settings/scans"},
		{http.MethodPut, "/api/v1/settings/dlna"},
		{http.MethodPut, "/api/v1/settings/jellyfin"},
		{http.MethodPut, "/api/v1/settings/trusted-https"},
		{http.MethodPost, "/api/v1/supporter/activate"},
		{http.MethodPost, "/api/v1/libraries"},
		{http.MethodDelete, "/api/v1/libraries"},
		{http.MethodPost, "/api/v1/profiles"},
		{http.MethodPut, "/api/v1/profiles/missing"},
		{http.MethodPut, "/api/v1/profiles/missing/password"},
		{http.MethodPost, "/api/v1/api-keys"},
		{http.MethodPost, "/api/v1/remote-access/wireguard"},
		{http.MethodDelete, "/api/v1/remote-access/wireguard"},
		{http.MethodPost, "/api/v1/viewing-imports/preview"},
		{http.MethodPost, "/api/v1/viewing-syncs"},
		{http.MethodPost, "/api/v1/quick-connect"},
		{http.MethodPost, "/api/v1/quick-connect/token"},
		{http.MethodPost, "/api/v1/session"},
		{http.MethodPost, "/api/v1/me/mfa/setup"},
		{http.MethodPut, "/api/v1/me/mfa"},
		{http.MethodDelete, "/api/v1/me/mfa"},
		{http.MethodPost, "/api/v1/passkeys/register/finish"},
		{http.MethodPost, "/api/v1/passkeys/login/finish"},
	}
	for _, mutation := range paths {
		for name, body := range map[string]string{"malformed JSON": "{", "unknown field": `{"unexpected":true}`, "trailing object": "{}\n{}"} {
			response := rawAPIRequest(t, handler, token, mutation.method, mutation.path, body)
			if response.Code < 400 {
				t.Fatalf("%s %s accepted %s: %d %q", mutation.method, mutation.path, name, response.Code, response.Body.String())
			}
		}
	}
}

func TestVersionedAPIReportsMissingResourcesAcrossCapabilities(t *testing.T) {
	libraryAPIFixture.VersionedAPIMissingResources(t)
}

func TestVersionedAPIReadModelsAreReachableThroughOwnerSession(t *testing.T) {
	handler, token := apiServer(t)
	for _, path := range []string{
		"/api/v1", "/api/v1/me", "/api/v1/openapi.json", "/api/v1/history", "/api/v1/playlists", "/api/v1/collections", "/api/v1/shows", "/api/v1/albums", "/api/v1/downloads",
		"/api/v1/settings", "/api/v1/updates", "/api/v1/supporter", "/api/v1/hardware", "/api/v1/configuration", "/api/v1/profiles", "/api/v1/devices", "/api/v1/api-keys", "/api/v1/backups", "/api/v1/diagnostics", "/api/v1/metrics", "/api/v1/maintenance", "/api/v1/marker-analysis", "/api/v1/remote-access", "/api/v1/activity", "/api/v1/viewing-syncs",
	} {
		response := apiCall(t, handler, token, http.MethodGet, path, nil)
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s = %d %q", path, response.Code, response.Body.String())
		}
	}
}

func firstAPIItemID(t *testing.T, handler http.Handler, token string) string {
	t.Helper()
	response := apiCall(t, handler, token, http.MethodGet, "/api/v1/library", nil)
	var library struct {
		Items []struct{ ID string }
	}
	mustJSON(t, response, &library)
	if len(library.Items) == 0 {
		t.Fatal("test Library has no items")
	}
	return library.Items[0].ID
}

func rawAPIRequest(t *testing.T, handler http.Handler, token, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
