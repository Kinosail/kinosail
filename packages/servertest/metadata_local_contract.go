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

// MetadataConfig contains only app constructor inputs exercised by the metadata contracts.
type MetadataConfig struct{ MediaDir, CacheDir, TMDBToken, TMDBURL, TMDBImageURL string }

// MetadataFixture binds shared metadata scenarios to real app configuration and provider fixtures.
type MetadataFixture struct {
	New     func(MetadataConfig) http.Handler
	IDFirst func(string, string, bool, *int, *int) http.HandlerFunc
}

// ViewerCanSeeLocalNfoMetadata checks metadata through the real app server.
func (fixture MetadataFixture) ViewerCanSeeLocalNfoMetadata(t *testing.T) {
	t.Helper()
	t.Parallel()

	mediaDir := t.TempDir()
	for name, content := range map[string]string{
		"Primer.2004.mp4": "video",
		"Primer.2004.nfo": `<movie><title>Primer</title><year>2004</year><plot>Two engineers discover time travel.</plot></movie>`,
	} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := fixture.New(MetadataConfig{MediaDir: mediaDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	match := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())
	if len(match) != 2 {
		t.Fatalf("rich metadata is not searchable: %q", home.Body.String())
	}
	id := match[1]
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))

	if !strings.Contains(home.Body.String(), "Primer") || !strings.Contains(home.Body.String(), "· 2004") || !strings.Contains(player.Body.String(), "Two engineers discover time travel.") {
		t.Fatalf("home = %q, player = %q", home.Body.String(), player.Body.String())
	}
}

// ViewerCanBrowseAndSearchRichLocalMetadata checks metadata through the real app server.
func (fixture MetadataFixture) ViewerCanBrowseAndSearchRichLocalMetadata(t *testing.T) {
	t.Helper()
	t.Parallel()

	mediaDir := t.TempDir()
	for name, content := range map[string]string{
		"Alien.mp4": "video",
		"Alien.nfo": `<movie><title>Alien</title><tagline>In space no one can hear you scream.</tagline><genre>Science Fiction</genre><genre>Horror</genre><director>Ridley Scott</director><studio>20th Century Fox</studio></movie>`,
	} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := fixture.New(MetadataConfig{MediaDir: mediaDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?q=Ridley", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))

	assertMetadataPlayer(t, player, "In space no one can hear you scream.", "Science Fiction", "Horror", "Ridley Scott", "20th Century Fox")
}

func assertMetadataPlayer(t *testing.T, player *httptest.ResponseRecorder, values ...string) {
	t.Helper()
	for _, value := range values {
		if !strings.Contains(player.Body.String(), value) {
			t.Fatalf("player has no %q: %q", value, player.Body.String())
		}
	}
}
