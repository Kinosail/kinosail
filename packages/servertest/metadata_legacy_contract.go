package servertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LegacyTMDBUsesFilenameIMDbIDBeforeTitleSearch checks metadata through the real app server.
func (fixture MetadataFixture) LegacyTMDBUsesFilenameIMDbIDBeforeTitleSearch(t *testing.T) {
	t.Helper()
	t.Parallel()
	finds, searches := 0, 0
	provider := httptest.NewServer(fixture.IDFirst("test-token", "/3", true, &finds, &searches))
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	name := "'Twas the Night Before Christmas (1974) {imdb tt0208654} [Bluray 1080p] [DTS 1 0][x264] SADPANDA.mkv"
	if err := os.WriteFile(filepath.Join(mediaDir, name), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.New(MetadataConfig{MediaDir: mediaDir, CacheDir: t.TempDir(), TMDBToken: "test-token", TMDBURL: provider.URL + "/3", TMDBImageURL: provider.URL})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if finds != 1 || searches != 0 || !strings.Contains(home.Body.String(), "&#39;Twas the Night Before Christmas") {
		t.Fatalf("finds = %d, searches = %d, home = %q", finds, searches, home.Body.String())
	}
}

// TMDBAttributionIsVisibleWhenEnabled checks metadata through the real app server.
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
