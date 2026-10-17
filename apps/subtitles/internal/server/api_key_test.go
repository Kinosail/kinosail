package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestScopedAPIKeysAreHashedEnforcedAndRevocable(t *testing.T) {
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Movie.mp4"), []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	create := requestWithCookie(t, handler, http.MethodPost, "/settings/api-keys", "name=Automation&scopes=library", owner)
	secret := servertest.APIKeySecret(create.Body.String())
	if create.Code != http.StatusCreated || secret == "" {
		t.Fatalf("create key = %d %q", create.Code, create.Body.String())
	}
	assertNoStore(t, create)
	stored := storedState(t, dataDir, "api_keys.json")
	if strings.Contains(string(stored), secret) {
		t.Fatalf("persisted keys expose secret: data=%q", stored)
	}
	handler = server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	library := apiKeyRequest(t, handler, "/api/v1/library", secret)
	match := regexp.MustCompile(`"id":"([a-f0-9]+)"`).FindStringSubmatch(library.Body.String())
	if library.Code != http.StatusOK || len(match) != 2 {
		t.Fatalf("library scope = %d %q", library.Code, library.Body.String())
	}
	id := match[1]
	mutation := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/subtitle-library/"+id+"/fetch", strings.NewReader(`{}`))
	mutation.Header.Set("Authorization", "Bearer "+secret)
	mutation.Header.Set("Content-Type", "application/json")
	mutated := httptest.NewRecorder()
	handler.ServeHTTP(mutated, mutation)
	if mutated.Code != http.StatusForbidden {
		t.Fatalf("library key mutation = %d %q", mutated.Code, mutated.Body.String())
	}
	stream := apiKeyRequest(t, handler, "/media/"+id, secret)
	if stream.Code != http.StatusForbidden {
		t.Fatalf("stream without scope = %d %q", stream.Code, stream.Body.String())
	}
	if admin := apiKeyRequest(t, handler, "/api/v1/settings", secret); admin.Code != http.StatusForbidden {
		t.Fatalf("administration without scope = %d %q", admin.Code, admin.Body.String())
	}
	if metrics := apiKeyRequest(t, handler, "/api/v1/metrics", secret); metrics.Code != http.StatusForbidden {
		t.Fatalf("metrics without scope = %d %q", metrics.Code, metrics.Body.String())
	}
	keys := apiCall(t, handler, owner.Value, http.MethodGet, "/api/v1/api-keys", nil)
	keyMatch := regexp.MustCompile(`"id":"([a-f0-9]+)"`).FindStringSubmatch(keys.Body.String())
	if keys.Code != http.StatusOK || len(keyMatch) != 2 || !strings.Contains(keys.Body.String(), `"name":"Automation"`) {
		t.Fatalf("keys = %d %q", keys.Code, keys.Body.String())
	}
	keyID := keyMatch[1]
	revoke := requestWithCookieRequest(t, http.MethodPost, "/settings/api-keys/revoke", "id="+keyID, owner)
	serveRequest(handler, revoke)
	if revoked := apiKeyRequest(t, handler, "/api/v1/library", secret); revoked.Code != http.StatusUnauthorized {
		t.Fatalf("revoked key = %d %q", revoked.Code, revoked.Body.String())
	}
}

func TestAPIKeyCreationRejectsUnknownScopes(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	response := requestWithCookie(t, handler, http.MethodPost, "/settings/api-keys", "name=Unsafe&scopes=unknown", owner)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "scope must be") {
		t.Fatalf("unknown scope = %d %q", response.Code, response.Body.String())
	}
}

func assertNoStore(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("response may be cached: %q", response.Header())
	}
}

func apiKeyRequest(t *testing.T, handler http.Handler, path, secret string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	request.Header.Set("Authorization", "Bearer "+secret)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
