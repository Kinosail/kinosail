package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestVersionedAPIFailsClosedOnInvalidCapabilityInputs(t *testing.T) {
	handler, token := apiServer(t)
	calls := []apiTestCall{
		{Method: http.MethodPut, Path: "/api/v1/me/language", Body: map[string]any{"language": "xx"}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/configuration/paths.data", Body: map[string]any{"value": "/elsewhere"}, Status: http.StatusConflict},
		{Method: http.MethodPut, Path: "/api/v1/configuration/server.name", Body: map[string]any{"value": "Restart name"}, Status: http.StatusConflict},
		{Method: http.MethodPut, Path: "/api/v1/settings/server", Body: map[string]any{"name": ""}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/session-timeouts", Body: map[string]any{"inactiveHours": 48, "absoluteHours": 24}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/subtitles", Body: map[string]any{"language": "invalid"}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/subtitles", Body: map[string]any{"language": "en", "preference": "unknown"}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/subtitles", Body: map[string]any{"languages": []string{}}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/subtitles", Body: map[string]any{"languages": []string{"pt-br", "pt-BR"}}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/subtitles", Body: map[string]any{"language": "en", "languages": []string{"es"}}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/scans", Body: map[string]any{"frequency": "invalid"}, Status: http.StatusBadRequest},
		{Method: http.MethodPost, Path: "/api/v1/libraries", Body: map[string]any{"path": "../outside"}, Status: http.StatusBadRequest},
		{Method: http.MethodDelete, Path: "/api/v1/libraries", Body: map[string]any{"path": "missing"}, Status: http.StatusBadRequest},
		{Method: http.MethodPost, Path: "/api/v1/api-keys", Body: map[string]any{"name": "Key", "scopes": "unknown"}, Status: http.StatusBadRequest},
		{Method: http.MethodPost, Path: "/api/v1/tasks/unknown", Body: nil, Status: http.StatusNotFound},
		{Method: http.MethodPost, Path: "/api/v1/backups", Body: nil, Status: http.StatusServiceUnavailable},
		{Method: http.MethodPost, Path: "/api/v1/backups/verify", Body: nil, Status: http.StatusServiceUnavailable},
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
		NoPlayback:  true,
	})
}
