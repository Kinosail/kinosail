package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestViewerCanBrowseDedicatedLibraryViews(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	for _, name := range []string{"Movie.mp4", "Show.S01E01.Pilot.mkv", "Song.mp3", "Photo.jpg"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	for view, expected := range map[string]string{"movies": "Movie", "shows": "Show", "music": "Song", "photos": "Photo"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view="+view, nil))
		body := response.Body.String()
		if !strings.Contains(body, expected) || !strings.Contains(body, `name="view" value="`+view+`"`) {
			t.Fatalf("%s view = %q", view, body)
		}
	}
}

func TestViewerCanSortALibraryByNewest(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	alpha, zulu := filepath.Join(mediaDir, "Alpha.mp4"), filepath.Join(mediaDir, "Zulu.mp4")
	for _, path := range []string{alpha, zulu} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(alpha, time.Unix(1, 0), time.Unix(1, 0)); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.New(server.Config{MediaDir: mediaDir}).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=movies&sort=added", nil))
	body := response.Body.String()

	if strings.Index(body, ">Zulu<") > strings.Index(body, ">Alpha<") || !strings.Contains(body, `<option value="added" selected>Newest`) {
		t.Fatalf("newest = %q", body)
	}
}
