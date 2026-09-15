package servertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AssertLocalizationDoesNotTranslateLibraryContent preserves localized chrome without changing media content.
func AssertLocalizationDoesNotTranslateLibraryContent(t *testing.T, fixture LibraryAPIFixture) {
	t.Parallel()
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Home.mp4"), []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(media, t.TempDir(), false)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?lang=es", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `>Inicio<`) || !strings.Contains(response.Body.String(), `<h2>Home</h2>`) {
		t.Fatalf("localized dynamic content = %d %q", response.Code, response.Body.String())
	}
}

// AssertLocalizedPlayerUsesCatalogedPlaybackMode preserves localized chrome without changing media content.
func AssertLocalizedPlayerUsesCatalogedPlaybackMode(t *testing.T, fixture LibraryAPIFixture) {
	t.Parallel()
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Arrival.mp4"), []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler, id := FirstWebItem(t, fixture.NewHandler(media, t.TempDir(), false))
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id+"?lang=es", nil))
	if !strings.Contains(player.Body.String(), ">Compatibilidad<") || strings.Contains(player.Body.String(), "Directa First") || strings.Contains(player.Body.String(), "Reproducción policy") {
		t.Fatalf("Spanish playback mode = %q", player.Body.String())
	}
}
