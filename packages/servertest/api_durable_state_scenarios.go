package servertest

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// APIDurableStateFixture binds real-app initialization and authentication.
type APIDurableStateFixture struct {
	NewHandler  func(string, string) http.Handler
	SignIn      func(*testing.T, http.Handler, string, string) *http.Cookie
	FirstItemID func(*testing.T, http.Handler, string) string
}

// APIDurableStateFailures checks failed persistence through the real API.
func APIDurableStateFailures(t *testing.T, fixture APIDurableStateFixture) {
	media, data := t.TempDir(), filepath.Join(t.TempDir(), "data")
	if err := os.Mkdir(data, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(media, "Movie.mp4"), []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(media, data)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	token := owner.Value
	itemID := fixture.FirstItemID(t, handler, token)
	if err := os.RemoveAll(data); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(data, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, call := range []APITestCall{
		{Method: http.MethodPut, Path: "/api/v1/items/" + itemID + "/progress", Body: map[string]any{"seconds": 10}, Status: http.StatusInternalServerError},
		{Method: http.MethodPut, Path: "/api/v1/items/" + itemID + "/list", Body: map[string]any{"listed": true}, Status: http.StatusInternalServerError},
		{Method: http.MethodPost, Path: "/api/v1/playlists", Body: map[string]any{"name": "Favorites"}, Status: http.StatusBadRequest},
		{Method: http.MethodPost, Path: "/api/v1/collections", Body: map[string]any{"name": "Favorites"}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/server", Body: map[string]any{"name": "Changed"}, Status: http.StatusBadRequest},
		{Method: http.MethodPut, Path: "/api/v1/settings/navigation", Body: map[string]any{"items": []string{"home", "movies"}}, Status: http.StatusBadRequest},
		{Method: http.MethodPost, Path: "/api/v1/profiles", Body: map[string]any{"name": "Viewer", "password": "viewer-password"}, Status: http.StatusBadRequest},
		{Method: http.MethodPost, Path: "/api/v1/api-keys", Body: map[string]any{"name": "Device", "scopes": "library"}, Status: http.StatusBadRequest},
		{Method: http.MethodDelete, Path: "/api/v1/me/oidc", Body: nil, Status: http.StatusBadRequest},
	} {
		response := APICall(t, handler, token, call.Method, call.Path, call.Body)
		if response.Code != call.Status {
			t.Fatalf("%s %s = %d %q", call.Method, call.Path, response.Code, response.Body.String())
		}
	}
}
