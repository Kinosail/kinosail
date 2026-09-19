package server_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestAPIParityContract(t *testing.T) {
	servertest.APIParityContract(t, servertest.APIParityFixture{
		Server: apiServer,
		Bootstrap: func(t *testing.T) http.Handler {
			return server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
		},
		TOTP: testTOTP,
	})
}

func TestProviderAPIFailuresAreExplicit(t *testing.T) {
	t.Parallel()

	handler, token := apiServer(t)
	var catalog struct {
		Items []struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		} `json:"items"`
	}
	mustJSON(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/library", nil), &catalog)
	id := ""
	for _, item := range catalog.Items {
		if item.Kind == "video" {
			id = item.ID
			break
		}
	}
	assertAPICalls(t, handler, token, []apiTestCall{
		{Method: http.MethodPost, Path: "/api/v1/items/" + id + "/metadata/refresh", Body: nil, Status: http.StatusBadGateway},
		{Method: http.MethodPost, Path: "/api/v1/items/" + id + "/subtitles", Body: map[string]any{"language": "en"}, Status: http.StatusBadGateway},
	})
}

func TestVersionedAPICoversBrowserOnlyCapabilities(t *testing.T) {
	t.Parallel()
	handler, token := apiServer(t)
	item := apiCall(t, handler, token, http.MethodGet, "/api/v1/library?sort=added", nil)
	assertAPIBody(t, item, http.StatusOK, `"tagline":"First contact"`, `"director":"Denis Villeneuve"`, `"studio":"Paramount"`, `"container":"MP4"`, `"size":5`, `"added":`)
	openapi := apiCall(t, handler, token, http.MethodGet, "/api/v1/openapi.json", nil)
	paths := []string{`/api/v1/passkeys/register/begin`, `/api/v1/session/oidc`, `/api/v1/watch-rooms/{id}/events`, `/api/v1/items/{id}/markers`, `/api/v1/items/{id}/playback-events`, `/api/v1/metadata/bulk`, `/api/v1/configuration/{key}`, `/api/v1/backups/verify`, `/api/v1/viewing-imports/preview`, `/api/v1/viewing-syncs/{id}/run`, `/api/v1/supporter/activate`, `/api/v1/supporter/certificates/{family}`}
	assertAPIBody(t, openapi, http.StatusOK, paths...)
	for _, path := range paths {
		if count := bytes.Count(openapi.Body.Bytes(), []byte(`"`+path+`"`)); count != 1 {
			t.Fatalf("OpenAPI path %s occurs %d times", path, count)
		}
	}
}
