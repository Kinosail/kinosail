package servertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (fixture LibraryAPIFixture) ClientCanAuthenticateAndBrowseLibraryWithoutFilesystemPaths(t *testing.T) {
	t.Parallel()

	mediaDir, dataDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(mediaDir, dataDir, true)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	var session struct {
		Token string `json:"token"`
	}
	session.Token = owner.Value
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil)
	request.Header.Set("Authorization", "Bearer "+session.Token)
	library := httptest.NewRecorder()
	handler.ServeHTTP(library, request)

	if session.Token == "" || library.Code != http.StatusOK || !strings.Contains(library.Body.String(), `"title":"Arrival"`) || strings.Contains(library.Body.String(), mediaDir) {
		t.Fatalf("library = %d %q", library.Code, library.Body.String())
	}
	request = httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/api/v1/session", nil)
	request.Header.Set("Authorization", "Bearer "+session.Token)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil)
	request.Header.Set("Authorization", "Bearer "+session.Token)
	revoked := httptest.NewRecorder()
	handler.ServeHTTP(revoked, request)
	if revoked.Code != http.StatusUnauthorized {
		t.Fatalf("revoked client library = %d", revoked.Code)
	}
}

func (fixture LibraryAPIFixture) ClientLibraryRequiresAuthentication(t *testing.T) {
	t.Parallel()

	handler := fixture.NewHandler("", t.TempDir(), true)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
	if response.Code != http.StatusUnauthorized || response.Header().Get("Content-Type") != "application/json" || !strings.Contains(response.Body.String(), `"error"`) {
		t.Fatalf("anonymous client library = %d %q %q", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}
}

func (fixture LibraryAPIFixture) APIRouterFailuresUseTheJSONContract(t *testing.T) {
	t.Parallel()
	handler, token := fixture.Server(t)
	for _, test := range []struct {
		method, path, allow string
		status              int
	}{{http.MethodGet, "/api/v1/nope", "", http.StatusNotFound}, {http.MethodPatch, "/api/v1/library", "GET, HEAD", http.StatusMethodNotAllowed}} {
		response := APICall(t, handler, token, test.method, test.path, nil)
		if response.Code != test.status || response.Header().Get("Content-Type") != "application/json" || response.Header().Get("Allow") != test.allow || !strings.Contains(response.Body.String(), `"error"`) {
			t.Fatalf("%s %s = %d %q %q", test.method, test.path, response.Code, response.Header().Get("Content-Type"), response.Body.String())
		}
	}
}
