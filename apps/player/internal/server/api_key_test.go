package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
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
	createRequest := requestWithCookieRequest(t, http.MethodPost, "/settings/api-keys", "name=Dashboard&scopes=library", owner)
	createRequest.Header.Set("Accept-Language", "ar")
	create := httptest.NewRecorder()
	handler.ServeHTTP(create, createRequest)
	secret := regexp.MustCompile(`ks_[A-Z2-7]+`).FindString(create.Body.String())
	if create.Code != http.StatusCreated || secret == "" {
		t.Fatalf("create key = %d %q", create.Code, create.Body.String())
	}
	assertAPIKeyCopyControls(t, create)
	stored := storedState(t, dataDir, "api_keys.json")
	if strings.Contains(string(stored), secret) {
		t.Fatalf("persisted keys expose secret: data=%q", stored)
	}
	handler = server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, RequireAuth: true})
	library := apiKeyRequest(t, handler, "/api/v1/library", secret)
	id := servertest.AssertLibraryKeyContract(t, handler, library, secret)
	servertest.AssertLibraryKeyCannotMutate(t, handler, id, secret)
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
	settings := requestWithCookie(t, handler, http.MethodGet, "/settings", "", owner)
	keyID := regexp.MustCompile(`name="id" value="([a-f0-9]+)">Revoke Dashboard`).FindStringSubmatch(settings.Body.String())[1]
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

func TestRemovingProfileRevokesItsAPIKeys(t *testing.T) {
	t.Parallel()
	libraryAPIFixture.RemovingProfileRevokesItsAPIKeys(t)
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

func assertAPIKeyCopyControls(t *testing.T, create *httptest.ResponseRecorder) {
	t.Helper()
	for _, expected := range []string{`lang="ar" dir="rtl"`, `مفتاح API المُنشأ`, `id="api-key-secret" dir="ltr"`, `readonly`, `data-copy-target="api-key-secret"`, `نسخ مفتاح API`, `data-copy-success hidden>تم النسخ.`, `data-copy-fallback hidden>حدّد «نسخ» في المتصفح.`, `class="copy-status" aria-live="polite"`} {
		if !strings.Contains(create.Body.String(), expected) {
			t.Fatalf("created key lacks copy control %q", expected)
		}
	}
	assertNoStore(t, create)
}
