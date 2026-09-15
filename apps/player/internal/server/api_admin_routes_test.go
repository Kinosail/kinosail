package server_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestOwnerSettingsAndOperationsAPIRoutes(t *testing.T) { //nolint:cyclop // One route contract test checks all Owner operations.
	t.Parallel()
	handler, token := apiServer(t)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"transcoder":"automatic"`, `"accelerator":"auto"`, `"onboardingPending":true`, `"updates":{"automatic":true,"state":"not-checked"`)
	assertAPICalls(t, handler, token, []apiTestCall{
		{Method: http.MethodPut, Path: "/api/v1/settings/onboarding", Body: map[string]any{"enabled": true}, Status: http.StatusOK},
		{Method: http.MethodPut, Path: "/api/v1/settings/updates", Body: map[string]any{"automatic": true}, Status: http.StatusOK},
		{Method: http.MethodPut, Path: "/api/v1/settings/transcoder", Body: map[string]any{"quality": "speed", "accelerator": "none", "toneMap": false}, Status: http.StatusOK},
		{Method: http.MethodPut, Path: "/api/v1/settings/subtitles", Body: map[string]any{"language": "es"}, Status: http.StatusOK},
		{Method: http.MethodPut, Path: "/api/v1/settings/scans", Body: map[string]any{"frequency": "off"}, Status: http.StatusOK},
		{Method: http.MethodPut, Path: "/api/v1/settings/dlna", Body: map[string]any{"enabled": false}, Status: http.StatusOK},
		{Method: http.MethodPost, Path: "/api/v1/libraries", Body: map[string]any{"path": "/outside"}, Status: http.StatusBadRequest},
		{Method: http.MethodDelete, Path: "/api/v1/libraries", Body: map[string]any{"path": "missing"}, Status: http.StatusBadRequest},
		{Method: http.MethodPost, Path: "/api/v1/tasks/metadata", Body: nil, Status: http.StatusNoContent},
		{Method: http.MethodPost, Path: "/api/v1/tasks/clear-cache", Body: nil, Status: http.StatusNoContent},
		{Method: http.MethodPost, Path: "/api/v1/tasks/missing", Body: nil, Status: http.StatusNotFound},
	})
	missing := apiCall(t, handler, token, http.MethodPost, "/api/v1/tasks/missing", nil)
	if missing.Header().Get("Content-Type") != "application/json" || !bytes.Contains(missing.Body.Bytes(), []byte(`"error":"not found"`)) {
		t.Fatalf("missing API route = %q %q", missing.Header().Get("Content-Type"), missing.Body.String())
	}
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/diagnostics", nil), http.StatusOK, `"libraryItems"`)
	metrics := apiCall(t, handler, token, http.MethodGet, "/api/v1/metrics", nil)
	servertest.AssertOperationalMetrics(t, metrics)
}

func TestOnboardingSettingsAPIRequiresAnExplicitBoolean(t *testing.T) {
	libraryAPIFixture.OnboardingSettingsRequireExplicitBoolean(t)
}
