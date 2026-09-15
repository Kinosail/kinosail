package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestPlayerBrandingIsUsedByTheWebAppShell(t *testing.T) {
	t.Parallel()

	handler := server.New(server.Config{})
	for _, test := range []struct {
		path   string
		must   []string
		legacy string
	}{
		{path: "/", must: []string{"<title>Kinosail · Kinosail Subtitles</title>"}, legacy: "<title>Kinosail · Kinosail</title>"},
		{path: "/login", must: []string{"<title>Sign in · Kinosail Subtitles</title>", "<h1>Kinosail Subtitles</h1>"}, legacy: "<title>Sign in · Kinosail</title>"},
		{path: "/setup", must: []string{"<title>Set up Kinosail Subtitles</title>", "<strong>Kinosail Subtitles</strong>"}, legacy: "Set up Kinosail</title>"},
		{path: "/manifest.webmanifest", must: []string{`"name":"Kinosail Subtitles"`, `"short_name":"Subtitles"`}, legacy: `"name":"Kinosail"`},
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
