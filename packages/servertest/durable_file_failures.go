package servertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/documentdb"
)

// InvalidDurableShapesFailClosed checks every malformed persisted document at startup.
func (fixture LibraryAPIFixture) InvalidDurableShapesFailClosed(t *testing.T) {
	t.Helper()
	for _, name := range []string{"settings.json", "metadata.json", "progress.json", "lists.json", "playlists.json", "playlist_order.json", "smart_playlists.json", "audit_key.json", "audit.jsonl"} {
		t.Run(name, func(t *testing.T) {
			data := t.TempDir()
			if err := os.WriteFile(filepath.Join(data, name), []byte(`"invalid shape"`), 0o600); err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			fixture.NewHandler("", data, false).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("invalid %s state = %d %q", name, response.Code, response.Body.String())
			}
		})
	}
}

// InvalidDownloadStateFailsClosed preserves the download path-traversal startup rejection.
func InvalidDownloadStateFailsClosed(t *testing.T, newHandler func(data, cache string) http.Handler) {
	t.Helper()
	cache := t.TempDir()
	root := filepath.Join(cache, "downloads")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "aaaaaaaaaaaaaaaa.json"), []byte(`{"id":"aaaaaaaaaaaaaaaa","extension":"../../outside"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	newHandler(t.TempDir(), cache).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("invalid download state = %d %q", response.Code, response.Body.String())
	}
}

// IndividualStateFileFailures checks each API mutation against a blocked durable store.
func (fixture LibraryAPIFixture) IndividualStateFileFailures(t *testing.T, firstItemID func(*testing.T, http.Handler, string) string) {
	t.Helper()
	tests := []struct {
		file, method, path string
		body               any
	}{
		{"sessions.json", http.MethodPost, "/api/v1/session", map[string]any{"name": "Owner", "password": "owner-password"}},
		{"sessions.json", http.MethodDelete, "/api/v1/session", nil},
		{"profiles.json", http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Viewer", "password": "viewer-password", "rating": "all", "libraries": "all"}},
		{"profiles.json", http.MethodPut, "/api/v1/profiles/{profile}", map[string]any{"rating": "all", "libraries": "all"}},
		{"profiles.json", http.MethodPut, "/api/v1/profiles/{profile}/password", map[string]any{"password": "changed-password"}},
		{"api_keys.json", http.MethodPost, "/api/v1/api-keys", map[string]any{"name": "Device", "scopes": "library"}},
		{"settings.json", http.MethodPut, "/api/v1/settings/server", map[string]any{"name": "Changed"}},
		{"settings.json", http.MethodPut, "/api/v1/settings/navigation", map[string]any{"items": []string{"home", "movies"}}},
		{"settings.json", http.MethodPut, "/api/v1/settings/mfa", map[string]any{"required": true}},
		{"settings.json", http.MethodPut, "/api/v1/settings/session-timeouts", map[string]any{"inactiveHours": 168, "absoluteHours": 720}},
		{"settings.json", http.MethodPut, "/api/v1/settings/playback", map[string]any{"mode": "direct", "subtitles": "on", "autoSkip": []string{"intro"}, "autoplay": true}},
		{"settings.json", http.MethodPut, "/api/v1/settings/transcoder", map[string]any{"quality": "speed", "accelerator": "none", "toneMap": false}},
		{"settings.json", http.MethodPut, "/api/v1/settings/subtitles", map[string]any{"language": "es"}},
		{"settings.json", http.MethodPut, "/api/v1/settings/scans", map[string]any{"frequency": "off"}},
		{"settings.json", http.MethodPut, "/api/v1/settings/jellyfin", map[string]any{"enabled": false}},
		{"updates.json", http.MethodPut, "/api/v1/settings/updates", map[string]any{"automatic": true}},
		{"lists.json", http.MethodPut, "/api/v1/items/{item}/list", map[string]any{"listed": true}},
		{"playlists.json", http.MethodPost, "/api/v1/playlists", map[string]any{"name": "Favorites"}},
		{"smart_playlists.json", http.MethodPost, "/api/v1/smart-playlists", map[string]any{"name": "Recent", "kind": "video", "sort": "added"}},
		{"progress.json", http.MethodPut, "/api/v1/items/{item}/progress", map[string]any{"seconds": 10}},
		{"metadata.json", http.MethodPut, "/api/v1/items/{item}/metadata", map[string]any{"title": "Changed", "year": "2026", "plot": "Plot", "rating": "PG", "tagline": "Tagline", "genres": "Drama"}},
	}
	for _, test := range tests {
		t.Run(test.method+"_"+test.file+"_"+test.path, func(t *testing.T) {
			handler, data, token, profile, item := fixture.durableFileFixture(t, firstItemID)
			blockStateFile(t, data, test.file)
			path := strings.Replace(test.path, "{profile}", profile, 1)
			path = strings.Replace(path, "{item}", item, 1)
			response := APICall(t, handler, token, test.method, path, test.body)
			if response.Code < http.StatusBadRequest {
				t.Fatalf("%s %s with blocked %s = %d %q", test.method, path, test.file, response.Code, response.Body.String())
			}
			assertBlockedStateFilePreserved(t, data)
		})
	}
}

func (fixture LibraryAPIFixture) durableFileFixture(t *testing.T, firstItemID func(*testing.T, http.Handler, string) string) (http.Handler, string, string, string, string) {
	t.Helper()
	media, data := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Movie.mp4"), []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(media, data, true)
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	token := owner.Value
	var profiles struct {
		Profiles []struct {
			ID string `json:"id"`
		} `json:"profiles"`
	}
	MustJSON(t, APICall(t, handler, token, http.MethodGet, "/api/v1/profiles", nil), &profiles)
	if len(profiles.Profiles) != 1 {
		t.Fatalf("profiles = %#v", profiles.Profiles)
	}
	return handler, data, token, profiles.Profiles[0].ID, firstItemID(t, handler, token)
}

func blockStateFile(t *testing.T, data, name string) {
	t.Helper()
	path := filepath.Join(data, documentdb.Filename)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		t.Fatalf("block %s: %v", name, err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("block %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(path, "keep"), []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertBlockedStateFilePreserved(t *testing.T, data string) {
	t.Helper()
	path := filepath.Join(data, documentdb.Filename)
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 1 || entries[0].Name() != "keep" {
		t.Fatalf("failed mutation changed the blocked database directory: %v, %v", entries, err)
	}
	value, err := os.ReadFile(filepath.Join(path, "keep"))
	if err != nil || string(value) != "blocked" {
		t.Fatalf("failed mutation changed the blocker sentinel: %q, %v", value, err)
	}
}
