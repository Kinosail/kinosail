package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestUnknownBrowserPageUsesTheKinosailErrorPage(t *testing.T) {
	t.Parallel()

	handler := newJellyfinServer(t, server.Config{MediaDir: t.TempDir()})
	cases := []struct {
		path, accept string
		branded      bool
	}{
		{"/no-such-page", "text/html,application/xhtml+xml", true},
		{"/settings/no-such-page", "text/html", true},
		{"/no-such-page", "*/*", false},
		{"/static/no-such-file.js", "*/*", false},
		{"/api/v1/no-such-endpoint", "text/html", false},
	}
	for _, test := range cases {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, test.path, nil)
		request.Header.Set("Accept", test.accept)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		body := response.Body.String()
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s %s = %d", test.path, test.accept, response.Code)
		}
		branded := strings.Contains(body, `<html lang="en"`) && strings.Contains(body, "<main") && strings.Contains(body, "This item could not be found.")
		if branded != test.branded {
			t.Fatalf("%s %s branded = %v: %.300s", test.path, test.accept, branded, body)
		}
		if test.branded && response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s cache = %q", test.path, response.Header().Get("Cache-Control"))
		}
	}
}
