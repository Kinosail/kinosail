package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestOwnerCanInspectMetricsAndRedactedDiagnostics(t *testing.T) {
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	failedRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/private-title?token=private-secret", nil)
	failedRequest.Header.Set("X-Request-ID", "550e8400-e29b-41d4-a716-446655440000")
	failed := httptest.NewRecorder()
	handler.ServeHTTP(failed, failedRequest)
	if failed.Code != http.StatusNotFound {
		t.Fatalf("failed request = %d", failed.Code)
	}

	system := getWithCookie(t, handler, "/settings/system", owner)
	activityAPI := getWithCookie(t, handler, "/api/v1/activity", owner)
	diagnosticsAPI := getWithCookie(t, handler, "/api/v1/diagnostics", owner)
	metrics := getWithCookie(t, handler, "/settings/metrics", owner)
	diagnostics := getWithCookie(t, handler, "/settings/diagnostics.json", owner)

	requireResponse(t, system, http.StatusOK, "Recent activity", "Diagnostics", "Download safe diagnostics", `class="activity-list"`, `class="activity-row"`)
	requireResponse(t, activityAPI, http.StatusOK, `"audit":`)
	requireResponse(t, diagnosticsAPI, http.StatusOK, `"libraryItems":1`, `"activityHealthy":true`, `"recentFailures":`, `"requestId":"550e8400-e29b-41d4-a716-446655440000"`, `"status":404`)
	requireResponse(t, metrics, http.StatusOK, "kinosail_library_items 1", "kinosail_http_requests_total", "kinosail_audit_write_failures_total")
	requireResponse(t, diagnostics, http.StatusOK, `"libraryItems":1`, `"httpRequests":`, `"activityHealthy":true`)
	if strings.Contains(system.Body.String(), "owner-password") || strings.Contains(system.Body.String(), mediaDir) {
		t.Fatalf("system page exposed sensitive data: %s", system.Body.String())
	}
	if diagnostics.Header().Get("Content-Disposition") == "" || strings.Contains(diagnostics.Body.String(), "owner-password") || strings.Contains(diagnostics.Body.String(), mediaDir) {
		t.Fatalf("unsafe diagnostics: headers=%v body=%s", diagnostics.Header(), diagnostics.Body.String())
	}
	for _, private := range []string{"private-title", "private-secret", "token="} {
		if strings.Contains(diagnosticsAPI.Body.String(), private) {
			t.Fatalf("diagnostics exposed %q: %s", private, diagnosticsAPI.Body.String())
		}
	}
}

func requireResponse(t *testing.T, response *httptest.ResponseRecorder, status int, values ...string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("response = %d, want %d: %s", response.Code, status, response.Body.String())
	}
	for _, value := range values {
		if !strings.Contains(response.Body.String(), value) {
			t.Fatalf("response missing %q: %s", value, response.Body.String())
		}
	}
}

func TestHealthEndpointStaysPublic(t *testing.T) {
	t.Parallel()
	response := httptest.NewRecorder()
	server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true}).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("health = %d headers=%v", response.Code, response.Header())
	}
}

func TestOwnerCanRunLibraryTask(t *testing.T) {
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	if err := os.WriteFile(filepath.Join(mediaDir, "Heat.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	run := requestWithCookie(t, handler, http.MethodPost, "/settings/tasks/scan", "", owner)
	home := getWithCookie(t, handler, "/", owner)
	if run.Code != http.StatusSeeOther || !strings.Contains(home.Body.String(), "Heat") {
		t.Fatalf("run=%d home=%s", run.Code, home.Body.String())
	}
}
