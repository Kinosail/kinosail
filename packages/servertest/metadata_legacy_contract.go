package servertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func (fixture MetadataFixture) ViewerKeepsMigratedLegacyTMDBMetadata(t *testing.T) {
	t.Helper()
	t.Parallel()
	mediaDir, dataDir, cacheDir := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Primer (2004).mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := MetadataConfig{MediaDir: mediaDir, DataDir: dataDir, CacheDir: cacheDir}
	id := legacyMetadataItemID(t, fixture.New(config))
	legacy := filepath.Join(cacheDir, "tmdb", id)
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "poster.jpg"), []byte("poster"), 0o600); err != nil {
		t.Fatal(err)
	}
	cached, err := json.Marshal(map[string]any{"Title": "Primer Remastered", "Year": "2004", "Plot": "Two engineers discover time travel.", "Genres": "Science Fiction", "Poster": filepath.Join(legacy, "poster.jpg"), "TMDBID": 14337, "Version": 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "metadata.json"), cached, 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		home := httptest.NewRecorder()
		fixture.New(config).ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
		for _, value := range []string{"Primer Remastered", "/art/" + id} {
			if !strings.Contains(home.Body.String(), value) {
				t.Fatalf("migrated home has no %q: %q", value, home.Body.String())
			}
		}
		if _, err := os.Stat(filepath.Join(cacheDir, "tmdb")); !os.IsNotExist(err) {
			t.Fatalf("legacy TMDB cache was kept after migration: %v", err)
		}
	}
}

func legacyMetadataItemID(t *testing.T, handler http.Handler) string {
	t.Helper()
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	match := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())
	if len(match) != 2 {
		t.Fatalf("home = %q", home.Body.String())
	}
	return match[1]
}

func (fixture MetadataFixture) TMDBAttributionIsVisibleWhenEnabled(t *testing.T) {
	t.Helper()
	t.Parallel()

	handler := fixture.New(MetadataConfig{MediaDir: t.TempDir(), CacheDir: t.TempDir(), TMDBToken: "test-token"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	for _, value := range []string{`href="https://www.themoviedb.org"`, "This product uses the TMDB API but is not endorsed or certified by TMDB."} {
		if !strings.Contains(response.Body.String(), value) {
			t.Fatalf("home has no TMDB attribution %q: %q", value, response.Body.String())
		}
	}
}
