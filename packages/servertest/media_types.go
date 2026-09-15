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

// ViewerCanBrowseAndOpenMusicAndPhotos verifies distinct browsing and playback surfaces.
func (fixture LibraryAPIFixture) ViewerCanBrowseAndOpenMusicAndPhotos(t *testing.T) {
	t.Parallel()
	mediaDir := t.TempDir()
	for name, content := range map[string]string{"Song.mp3": "music", "Song.nfo": "<song><artist>Massive Attack</artist><album>Mezzanine</album><track>1</track></song>", "Vacation.jpg": "photo"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := fixture.NewHandler(mediaDir, "", false)
	music, photos := httptest.NewRecorder(), httptest.NewRecorder()
	handler.ServeHTTP(music, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=music", nil))
	handler.ServeHTTP(photos, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=photos", nil))
	albumID := regexp.MustCompile(`/album/([a-f0-9]+)`).FindStringSubmatch(music.Body.String())[1]
	album := httptest.NewRecorder()
	handler.ServeHTTP(album, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/album/"+albumID, nil))
	body := music.Body.String() + photos.Body.String() + album.Body.String()
	MustContainAll(t, body, "Albums", "Photos", "Massive Attack")
	links := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindAllStringSubmatch(body, -1)
	unique := make(map[string]bool)
	for _, link := range links {
		unique[link[1]] = true
	}
	if len(unique) != 2 {
		t.Fatalf("unique watch links = %v", unique)
	}
	var players strings.Builder
	for id := range unique {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
		if strings.Contains(response.Body.String(), `class="viewer"`) && strings.Contains(response.Body.String(), `/static/player.js`) {
			t.Fatal("photo page loaded the media playback script")
		}
		players.WriteString(response.Body.String())
	}
	MustContainAll(t, players.String(), "<audio", `/static/player.js`, `class="viewer"`, `class="player-page"`, "Mezzanine")
}

// MustContainAll asserts that value includes every expected fragment.
func MustContainAll(t *testing.T, value string, expected ...string) {
	t.Helper()
	for _, text := range expected {
		if !strings.Contains(value, text) {
			t.Fatalf("%q does not contain %q", value, text)
		}
	}
}
