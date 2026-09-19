package servertest

import (
	"net/http"
	"strings"
	"testing"
)

// APISecurityContract verifies the versioned API's positive path and fail-closed boundary.
func APISecurityContract(t *testing.T, server func(*testing.T) (http.Handler, string)) {
	t.Helper()
	handler, token := server(t)

	response := APICall(t, handler, token, http.MethodGet, "/api/v1/library", nil)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("authenticated API route = %d %q %q", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}

	for name, credential := range map[string]string{
		"anonymous":       "",
		"invalid session": "not-a-valid-session",
	} {
		t.Run(name, func(t *testing.T) {
			response := APICall(t, handler, credential, http.MethodGet, "/api/v1/library", nil)
			if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), "authentication required") {
				t.Fatalf("protected API route = %d %q", response.Code, response.Body.String())
			}
		})
	}

	response = APICall(t, handler, "", http.MethodPost, "/api/v1/profiles", map[string]any{
		"name": "Unauthenticated", "password": "not-created", "rating": "all", "libraries": []string{"all"},
	})
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous API mutation = %d %q", response.Code, response.Body.String())
	}

	response = APICall(t, handler, "", http.MethodGet, "/api/v1/not-registered", nil)
	if response.Code != http.StatusNotFound || response.Header().Get("Content-Type") != "application/json" || strings.Contains(response.Body.String(), "<!doctype") {
		t.Fatalf("anonymous unknown API route = %d %q %q", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}

	response = APICall(t, handler, token, http.MethodGet, "/api/v1/not-registered", nil)
	if response.Code != http.StatusNotFound || response.Header().Get("Content-Type") != "application/json" || strings.Contains(response.Body.String(), "<!doctype") {
		t.Fatalf("unknown API route = %d %q %q", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}

	response = APICall(t, handler, token, http.MethodPost, "/api/v1/library", nil)
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, HEAD" || response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("unsupported API method = %d allow=%q content-type=%q body=%q", response.Code, response.Header().Get("Allow"), response.Header().Get("Content-Type"), response.Body.String())
	}
}
