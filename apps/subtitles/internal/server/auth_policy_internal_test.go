package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPublicRoutePolicyIsExactAndDefaultsProtected(t *testing.T) {
	t.Parallel()
	data := t.TempDir()
	auth := newAuthentication(t.Context(), data, true, "", newSettingsStore("", data, "", nil), NotificationConfig{}, nil)
	public := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/passkeys/login/begin", nil)
	capability := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/Videos/media/stream?playSessionId=play", nil)
	noCapability := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/Videos/media/stream", nil)

	if !auth.public(public, "POST /api/v1/passkeys/login/begin") || !auth.public(capability, "GET /Videos/{id}/{stream}") {
		t.Fatal("explicit public routes were not public")
	}
	for _, pattern := range []string{
		"POST /api/v1/passkeys/register/begin",
		"POST /auth/passkeys/register/begin",
		"GET /static/{file...}",
		"GET /api/v1/library",
	} {
		if auth.public(public, pattern) {
			t.Fatalf("unlisted route %q was public", pattern)
		}
	}
	if auth.public(noCapability, "GET /Videos/{id}/{stream}") {
		t.Fatal("playback capability route was public without its capability")
	}
}

func TestViewerRemotePolicyUsesServerRequestContext(t *testing.T) {
	t.Parallel()
	profile := viewerProfile{}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.Header.Set("X-Kinosail-Remote", "true")
	if !profile.Allowed(publicInternetRequest(request), time.Now()) {
		t.Fatal("caller-controlled remote header changed Viewer policy")
	}
	var public *http.Request
	Remote(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) { public = request })).ServeHTTP(httptest.NewRecorder(), request)
	if profile.Allowed(publicInternetRequest(public), time.Now()) {
		t.Fatal("server-marked public request bypassed Viewer policy")
	}
}

func TestAnonymousRequestsReachOnlyExplicitPublicRoutes(t *testing.T) {
	t.Parallel()
	data := t.TempDir()
	if err := os.WriteFile(filepath.Join(data, "settings.json"), []byte(`{"name":"Kinosail","libraries":["."],"jellyfinCompatibility":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := New(Config{DataDir: data, RequireAuth: true, Configuration: jellyfinRouteConfiguration(t, data)})
	tests := []struct {
		method, path string
		status       int
		location     string
	}{
		{http.MethodGet, "/healthz", http.StatusOK, ""},
		{http.MethodGet, "/api/v1/library", http.StatusUnauthorized, ""},
		{http.MethodGet, "/settings", http.StatusSeeOther, "/setup"},
		{http.MethodPost, "/api/v1/passkeys/register/begin", http.StatusUnauthorized, ""},
		{http.MethodPost, "/auth/passkeys/register/begin", http.StatusSeeOther, "/setup"},
		{http.MethodGet, "/Videos/media/stream", http.StatusUnauthorized, ""},
		{http.MethodGet, "/Videos/media/stream?playSessionId=invalid", http.StatusNotFound, ""},
		{http.MethodGet, "/dlna/invalid/device.xml", http.StatusNotFound, ""},
		{http.MethodGet, "/static/not-registered.js", http.StatusNotFound, ""},
		{http.MethodGet, "/not-a-route", http.StatusNotFound, ""},
	}
	for _, test := range tests {
		request := httptest.NewRequestWithContext(t.Context(), test.method, test.path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.status || response.Header().Get("Location") != test.location {
			t.Errorf("%s %s = %d location %q, want %d %q", test.method, test.path, response.Code, response.Header().Get("Location"), test.status, test.location)
		}
	}
}

func TestEveryPublicPolicyNamesARegisteredRoute(t *testing.T) {
	t.Parallel()
	registered := routeSet(registeredRouteInventory(t)...)
	if len(registered) < 200 {
		t.Fatalf("route inventory found only %d routes", len(registered))
	}
	for pattern := range publicRoutes {
		if !registered[pattern] {
			t.Errorf("public policy names unregistered route %q", pattern)
		}
	}
}
