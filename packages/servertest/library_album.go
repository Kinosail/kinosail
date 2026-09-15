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

// ViewerCanBrowseAlbumTracksInOrder verifies metadata track ordering in the album view.
func (fixture LibraryAPIFixture) ViewerCanBrowseAlbumTracksInOrder(t *testing.T) {
	mediaDir := t.TempDir()
	for name, content := range map[string]string{
		"Late.mp3": "audio", "Late.nfo": "<song><title>Late</title><artist>Artist</artist><album>Record</album><track>2</track></song>",
		"First.mp3": "audio", "First.nfo": "<song><title>First</title><artist>Artist</artist><album>Record</album><track>1</track></song>",
	} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := fixture.NewHandler(mediaDir, "", false)
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=music", nil))
	id := regexp.MustCompile(`/album/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	album := httptest.NewRecorder()
	handler.ServeHTTP(album, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/album/"+id, nil))
	body := album.Body.String()
	if album.Code != http.StatusOK || !strings.Contains(body, "First") || !strings.Contains(body, "Late") || strings.Index(body, "First") > strings.Index(body, "Late") {
		t.Fatalf("album = %d %q", album.Code, body)
	}
}
