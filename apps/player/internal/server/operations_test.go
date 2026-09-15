package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestOwnerCanInspectMetricsPlaybackAndRedactedDiagnostics(t *testing.T) {
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	viewer := addAndSignInViewer(t, handler, owner, "Sam", "viewer-password")
	home := getWithCookie(t, handler, "/", owner)
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	requestWithCookie(t, handler, http.MethodPost, "/progress/"+id, "seconds=90", owner)

	settings := getWithCookie(t, handler, "/settings", owner)
	system := getWithCookie(t, handler, "/settings/system", owner)
	activityAPI := getWithCookie(t, handler, "/api/v1/activity", owner)
	diagnosticsAPI := getWithCookie(t, handler, "/api/v1/diagnostics", owner)
	metrics := getWithCookie(t, handler, "/settings/metrics", owner)
	diagnostics := getWithCookie(t, handler, "/settings/diagnostics.json", owner)
	denied, deniedDiagnostics := getWithCookie(t, handler, "/settings/system", viewer), getWithCookie(t, handler, "/settings/diagnostics.json", viewer)

	requireResponse(t, settings, http.StatusOK, `href="/settings/system"`, "Open System")
	requireResponse(t, system, http.StatusOK, "Recent activity", "Recent playback", "Diagnostics", "Arrival", "Download safe diagnostics", `class="activity-list"`, `class="activity-row"`)
	requireResponse(t, activityAPI, http.StatusOK, `"playback":`, `"progress":`, `"title":"Arrival"`)
	requireResponse(t, diagnosticsAPI, http.StatusOK, `"libraryItems":1`, `"activityHealthy":true`)
	requireResponse(t, metrics, http.StatusOK, "kinosail_library_items 1", "kinosail_http_requests_total", "kinosail_audit_write_failures_total")
	requireResponse(t, diagnostics, http.StatusOK, `"libraryItems":1`, `"httpRequests":`, `"activityHealthy":true`)
	requireResponse(t, denied, http.StatusForbidden)
	requireResponse(t, deniedDiagnostics, http.StatusForbidden)
	_, logs, found := strings.Cut(system.Body.String(), `id="logs"`)
	logs, _, closed := strings.Cut(logs, `id="playback"`)
	if !found || !closed || strings.Contains(logs, "playback.started") || strings.Contains(system.Body.String(), "owner-password") || strings.Contains(system.Body.String(), mediaDir) {
		t.Fatalf("system page mixed playback into logs or exposed sensitive data: %s", system.Body.String())
	}
	if diagnostics.Header().Get("Content-Disposition") == "" || strings.Contains(diagnostics.Body.String(), "owner-password") || strings.Contains(diagnostics.Body.String(), mediaDir) {
		t.Fatalf("unsafe diagnostics: headers=%v body=%s", diagnostics.Header(), diagnostics.Body.String())
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

func addAndSignInViewer(t *testing.T, handler http.Handler, owner *http.Cookie, name, password string) *http.Cookie {
	t.Helper()
	requestWithCookie(t, handler, http.MethodPost, "/settings/profiles", "name="+name+"&password="+password, owner)
	return signInTestProfile(t, handler, "/login", "name="+name+"&password="+password)
}

func TestHealthEndpointStaysPublic(t *testing.T) {
	t.Parallel()
	response := httptest.NewRecorder()
	server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true}).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("health = %d headers=%v", response.Code, response.Header())
	}
}

func TestOnlyOwnerCanRunLibraryTask(t *testing.T) {
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	viewer := addAndSignInViewer(t, handler, owner, "Sam", "viewer-password")
	if err := os.WriteFile(filepath.Join(mediaDir, "Heat.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	denied := requestWithCookie(t, handler, http.MethodPost, "/settings/tasks/scan", "", viewer)
	run := requestWithCookie(t, handler, http.MethodPost, "/settings/tasks/scan", "", owner)
	home := getWithCookie(t, handler, "/", owner)
	if denied.Code != http.StatusForbidden || run.Code != http.StatusSeeOther || !strings.Contains(home.Body.String(), "Heat") {
		t.Fatalf("denied=%d run=%d home=%s", denied.Code, run.Code, home.Body.String())
	}
}
