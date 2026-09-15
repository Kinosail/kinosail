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
		{Method: http.MethodPut, Path: "/api/v1/settings/subtitles", Body: map[string]any{"language": "es", "preference": "sdh"}, Status: http.StatusOK},
		{Method: http.MethodPut, Path: "/api/v1/settings/subtitles", Body: map[string]any{"languages": []string{"en", "pt-br", "zh-cn", "es-419"}}, Status: http.StatusOK},
		{Method: http.MethodPut, Path: "/api/v1/settings/scans", Body: map[string]any{"frequency": "off"}, Status: http.StatusOK},
		{Method: http.MethodPut, Path: "/api/v1/settings/dlna", Body: map[string]any{"enabled": false}, Status: http.StatusOK},
		{Method: http.MethodPost, Path: "/api/v1/libraries", Body: map[string]any{"path": "/outside"}, Status: http.StatusBadRequest},
		{Method: http.MethodDelete, Path: "/api/v1/libraries", Body: map[string]any{"path": "missing"}, Status: http.StatusBadRequest},
		{Method: http.MethodPost, Path: "/api/v1/tasks/metadata", Body: nil, Status: http.StatusNoContent},
		{Method: http.MethodPost, Path: "/api/v1/tasks/clear-cache", Body: nil, Status: http.StatusNoContent},
		{Method: http.MethodPost, Path: "/api/v1/tasks/missing", Body: nil, Status: http.StatusNotFound},
	})
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"subtitleLanguages":["en","pt-BR","zh-Hans","es-419"]`, `"subtitleLanguageCatalog"`, `"tag":"pt-BR"`)
	missing := apiCall(t, handler, token, http.MethodPost, "/api/v1/tasks/missing", nil)
	if missing.Header().Get("Content-Type") != "application/json" || !bytes.Contains(missing.Body.Bytes(), []byte(`"error":"not found"`)) {
		t.Fatalf("missing API route = %q %q", missing.Header().Get("Content-Type"), missing.Body.String())
	}
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/diagnostics", nil), http.StatusOK, `"libraryItems"`)
	metrics := apiCall(t, handler, token, http.MethodGet, "/api/v1/metrics", nil)
	checkMetrics := servertest.AssertOperationalMetrics
	checkMetrics(t, metrics)
}

func TestOnboardingSettingsAPIRequiresAnExplicitBoolean(t *testing.T) {
	scenario := libraryAPIFixture.OnboardingSettingsRequireExplicitBoolean
	scenario(t)
}
