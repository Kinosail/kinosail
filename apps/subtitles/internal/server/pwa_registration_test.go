package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestPWAHandlesServiceWorkerRegistrationFailure(t *testing.T) {
	response := httptest.NewRecorder()
	server.New(server.Config{}).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/pwa.js", nil))
	script := response.Body.String()
	if !strings.Contains(script, `serviceWorker.register("/service-worker.js?v=56").then`) || !strings.Contains(script, `.catch(() => {});`) {
		t.Fatalf("service worker registration failure is unhandled: %q", script)
	}
}
