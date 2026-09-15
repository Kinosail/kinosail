package servertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ViewerCanSeeTMDBMovieMetadataAndArtwork checks metadata through the real app server.
func (fixture MetadataFixture) ViewerCanSeeTMDBMovieMetadataAndArtwork(t *testing.T) {
	t.Helper()
	t.Parallel()

	provider := httptest.NewServer(http.HandlerFunc(serveMetadataTMDB))
	defer provider.Close()

	mediaDir := t.TempDir()
	cacheDir := t.TempDir()
	for name, content := range map[string]string{"Primer (2004).mp4": "video", "Primer (2004).nfo": "<movie><plot>Locally curated plot.</plot></movie>"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := fixture.New(MetadataConfig{MediaDir: mediaDir, CacheDir: cacheDir, TMDBToken: "test-token", TMDBURL: provider.URL + "/3", TMDBImageURL: provider.URL + "/images"})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	match := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())
	if len(match) != 2 || !strings.Contains(home.Body.String(), ">Primer<") || !strings.Contains(home.Body.String(), "· 2004") || !strings.Contains(home.Body.String(), `/art/`+match[1]) {
		t.Fatalf("home = %q", home.Body.String())
	}

	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+match[1], nil))
	assertMetadataPlayer(t, player, "Locally curated plot.", "Science Fiction", "Directed by Shane Carruth", "Shane Carruth", "Aaron", "/person/"+match[1]+"/0")

	headshot := httptest.NewRecorder()
	handler.ServeHTTP(headshot, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/person/"+match[1]+"/0", nil))
	if headshot.Code != http.StatusOK || headshot.Body.String() != "headshot" {
		t.Fatalf("headshot = %d %q", headshot.Code, headshot.Body.String())
	}

	provider.Close()
	offline := fixture.New(MetadataConfig{MediaDir: mediaDir, CacheDir: cacheDir})
	assertCachedTMDBMetadata(t, offline)
}

func assertCachedTMDBMetadata(t *testing.T, offline http.Handler) {
	t.Helper()
	cached := httptest.NewRecorder()
	offline.ServeHTTP(cached, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if !strings.Contains(cached.Body.String(), ">Primer<") || !strings.Contains(cached.Body.String(), "· 2004") || !strings.Contains(cached.Body.String(), "This product uses the TMDB API but is not endorsed or certified by TMDB.") {
		t.Fatalf("cached home = %q", cached.Body.String())
	}
}
