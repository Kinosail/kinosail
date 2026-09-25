package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestPlayerBrandingIsUsedByTheWebAppShell(t *testing.T) {
	t.Parallel()

	handler := server.New(server.Config{})
	for _, test := range []struct {
		path   string
		must   []string
		legacy string
	}{
		{path: "/", must: []string{"<title>Kinosail · Kinosail Player</title>"}, legacy: "<title>Kinosail · Kinosail</title>"},
		{path: "/login", must: []string{"<title>Sign in · Kinosail Player</title>", "<h1>Kinosail Player</h1>"}, legacy: "<title>Sign in · Kinosail</title>"},
		{path: "/setup", must: []string{"<title>Set up Kinosail Player</title>", "<strong>Kinosail Player</strong>"}, legacy: "Set up Kinosail</title>"},
		{path: "/manifest.webmanifest", must: []string{`"name":"Kinosail Player"`, `"short_name":"Kinosail Player"`}, legacy: `"name":"Kinosail"`},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, test.path, nil))
		body := response.Body.String()
		for _, expected := range test.must {
			if !strings.Contains(body, expected) {
				t.Errorf("%s body does not contain %q", test.path, expected)
			}
		}
		if strings.Contains(body, test.legacy) {
			t.Errorf("%s body contains legacy branding %q", test.path, test.legacy)
		}
	}
}

func TestWebBetaBadgeAppearsInSharedBrowserHeader(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{})
	for _, path := range []string{"/", "/settings"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusOK || strings.Count(response.Body.String(), `class="web-beta-badge">Beta</span>`) != 1 {
			t.Errorf("%s: expected one web beta badge in the browser header, got status %d", path, response.Code)
		}
	}
}
