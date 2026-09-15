package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ApplicationPagesExposeSharedKeyboardShortcuts runs the corresponding app regression contract.
func ApplicationPagesExposeSharedKeyboardShortcuts(t *testing.T, newHandler func(string) http.Handler, bundleVersion, navigationGuard string) {
	t.Parallel()

	handler := newHandler("")
	for _, path := range []string{"/settings/configuration", "/offline-downloads", "/settings/system"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "/static/main.kinosail.bundle.js?v="+bundleVersion) {
			t.Fatalf("%s = %d %q", path, response.Code, response.Body.String())
		}
	}
	bundle := httptest.NewRecorder()
	handler.ServeHTTP(bundle, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/main.kinosail.bundle.js", nil))
	for _, expected := range []string{"Keyboard shortcuts", `event.key === "?"`, `event.key.toLowerCase() === "k"`, navigationGuard, `event.key.toLowerCase() === "f"`} {
		if !strings.Contains(bundle.Body.String(), expected) {
			t.Fatalf("shared shortcut bundle missing %q", expected)
		}
	}
}
