package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestVersionedAPIFailsClosedOnInvalidCapabilityInputs(t *testing.T) { //nolint:funlen // The public failure table documents validation shared by API and web adapters.
	handler, token := apiServer(t)
	itemID := firstAPIItemID(t, handler, token)
	calls := []apiTestCall{
		{Method: http.MethodPut, Path: "/api/v1/me/language", Body: map[string]any{"language": "xx"}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/items/" + itemID + "/progress", Body: map[string]any{"seconds": -1}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/items/" + itemID + "/progress", Body: map[string]any{"seconds": 1_000_000_001}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/items/" + itemID + "/progress", Body: map[string]any{"seconds": 42, "session": strings.Repeat("s", 129), "revision": 1}, Status: http.StatusBadRequest},
		{Method: http.MethodPost, Path: "/api/v1/playlists", Body: map[string]any{"name": ""}, Status: http.StatusBadRequest},
		{Method: http.MethodPost, Path: "/api/v1/smart-playlists", Body: map[string]any{"name": "", "kind": "unknown", "sort": "unknown"}, Status: http.StatusBadRequest},
		{Method: http.MethodPost, Path: "/api/v1/smart-playlists", Body: map[string]any{"name": "Recent", "query": strings.Repeat("x", 201), "sort": "title"}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/playlists/Missing/items/" + itemID, Body: map[string]any{"included": true}, Status: http.StatusNotFound},
		{Method: http.MethodPut, Path: "/api/v1/playlists/Missing/order", Body: map[string]any{"ids": []string{itemID}}, Status: http.StatusBadRequest},
		{Method: http.MethodPost, Path: "/api/v1/collections", Body: map[string]any{"name": ""}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/collections/Missing/items/" + itemID, Body: map[string]any{"included": true}, Status: http.StatusNotFound},
		{Method: http.MethodPut, Path: "/api/v1/items/" + itemID + "/metadata", Body: map[string]any{}, Status: http.StatusBadRequest},
		{Method: http.MethodPost, Path: "/api/v1/items/" + itemID + "/metadata/refresh", Body: nil, Status: http.StatusBadGateway},
		{Method: http.MethodPost, Path: "/api/v1/items/" + itemID + "/subtitles", Body: map[string]any{"language": "invalid"}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/items/" + itemID + "/markers", Body: map[string]any{"type": "unknown", "start": 10, "end": 5}, Status: http.StatusBadRequest},
		{Method: http.MethodDelete, Path: "/api/v1/items/" + itemID + "/markers/unknown", Body: nil, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/configuration/paths.data", Body: map[string]any{"value": "/elsewhere"}, Status: http.StatusConflict},
		{Method: http.MethodPut, Path: "/api/v1/configuration/server.name", Body: map[string]any{"value": "Restart name"}, Status: http.StatusConflict},
		{Method: http.MethodPut, Path: "/api/v1/settings/server", Body: map[string]any{"name": ""}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/session-timeouts", Body: map[string]any{"inactiveHours": 48, "absoluteHours": 24}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/navigation", Body: map[string]any{"items": []string{"home", "home"}}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/playback", Body: map[string]any{"mode": "invalid", "subtitles": "invalid", "autoSkip": []string{"invalid"}}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/transcoder", Body: map[string]any{"quality": "invalid", "accelerator": "invalid"}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/subtitles", Body: map[string]any{"language": "invalid"}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/subtitles", Body: map[string]any{"language": "en", "preference": "unknown"}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/subtitles", Body: map[string]any{"languages": []string{}}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/subtitles", Body: map[string]any{"languages": []string{"pt-br", "pt-BR"}}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/subtitles", Body: map[string]any{"language": "en", "languages": []string{"es"}}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/scans", Body: map[string]any{"frequency": "invalid"}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/dlna", Body: map[string]any{"enabled": true}, Status: http.StatusBadRequest},
		{Method: http.MethodPost, Path: "/api/v1/libraries", Body: map[string]any{"path": "../outside"}, Status: http.StatusBadRequest},
		{Method: http.MethodDelete, Path: "/api/v1/libraries", Body: map[string]any{"path": "missing"}, Status: http.StatusBadRequest},
		{Method: http.MethodPost, Path: "/api/v1/profiles", Body: map[string]any{"name": "", "password": "short"}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/profiles/missing", Body: map[string]any{"rating": "all"}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/profiles/missing/password", Body: map[string]any{"password": "new-long-password"}, Status: http.StatusBadRequest},
		{Method: http.MethodPost, Path: "/api/v1/api-keys", Body: map[string]any{"name": "Key", "scopes": "unknown"}, Status: http.StatusBadRequest},
		{Method: http.MethodPost, Path: "/api/v1/tasks/unknown", Body: nil, Status: http.StatusNotFound},
		{Method: http.MethodPost, Path: "/api/v1/backups", Body: nil, Status: http.StatusServiceUnavailable},
		{Method: http.MethodPost, Path: "/api/v1/backups/verify", Body: nil, Status: http.StatusServiceUnavailable},
		{Method: http.MethodPost, Path: "/api/v1/viewing-imports/preview", Body: map[string]any{"source": "unknown"}, Status: http.StatusBadRequest},
		{Method: http.MethodPost, Path: "/api/v1/viewing-syncs", Body: map[string]any{"previewId": "missing", "interval": "now"}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/me/mfa", Body: map[string]any{"code": "000000"}, Status: http.StatusBadRequest},
		{Method: http.MethodDelete, Path: "/api/v1/me/mfa", Body: map[string]any{"code": "000000"}, Status: http.StatusUnauthorized},
	}
	assertAPICalls(t, handler, token, calls)
}

func TestVersionedAPISurfacesDurableStateFailures(t *testing.T) {
	servertest.APIDurableStateFailures(t, servertest.APIDurableStateFixture{
		NewHandler: func(media, data string) http.Handler {
			return server.New(server.Config{MediaDir: media, DataDir: data, RequireAuth: true})
		},
		SignIn:      signInTestProfile,
		FirstItemID: firstAPIItemID,
	})
}
