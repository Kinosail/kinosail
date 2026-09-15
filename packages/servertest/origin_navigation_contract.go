package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// AssertLoopbackNavigation checks that a loopback origin does not redirect the browser.
func AssertLoopbackNavigation(t *testing.T, handler http.Handler, target string) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	request.Header.Set("Sec-Fetch-Dest", "document")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Location") != "" {
		t.Fatalf("loopback browser = %d, location = %q", response.Code, response.Header().Get("Location"))
	}
}

// AssertOriginRedirectPrecedesStorageFailure checks navigation before the app reaches broken storage.
func AssertOriginRedirectPrecedesStorageFailure(t *testing.T, handler http.Handler, target, destination, label string) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	request.Header.Set("Sec-Fetch-Dest", "document")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusTemporaryRedirect || response.Header().Get("Location") != destination {
		t.Fatalf("%s = %d, location = %q", label, response.Code, response.Header().Get("Location"))
	}
}

// AssertOriginRedirectSanitizesUnsafePaths checks the real origin operation rejects unsafe destinations.
func AssertOriginRedirectSanitizesUnsafePaths(t *testing.T, alias, origin, label string, requireOrigin func(http.ResponseWriter, *http.Request) bool) {
	t.Helper()
	for name, path := range map[string]string{
		"scheme relative": "//attacker.example/path",
		"oversized":       "/" + strings.Repeat("a", 2049),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, alias+"/", nil)
			request.URL.Path = path
			response := httptest.NewRecorder()
			if requireOrigin(response, request) || response.Code != http.StatusTemporaryRedirect || response.Header().Get("Location") != origin+"/" {
				t.Fatalf("%s = %d, location = %q", label, response.Code, response.Header().Get("Location"))
			}
		})
	}
}
