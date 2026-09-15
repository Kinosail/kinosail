package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestViewerCanMarkMediaWatched(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Heat.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	dataDir := t.TempDir()
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/watched/"+id, strings.NewReader("watched=true"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	player := httptest.NewRecorder()
	server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir}).ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))

	if !strings.Contains(player.Body.String(), "Mark unwatched") {
		t.Fatalf("player = %q", player.Body.String())
	}
}

func TestViewerCanFilterTheirUnwatchedMedia(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	for _, name := range []string{"Arrival.mp4", "Heat.mp4"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir()})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?q=Heat", nil))
	heatID := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/watched/"+heatID, strings.NewReader("watched=true"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	filtered := httptest.NewRecorder()
	handler.ServeHTTP(filtered, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=unwatched", nil))

	if strings.Contains(filtered.Body.String(), ">Heat<") || !strings.Contains(filtered.Body.String(), ">Arrival<") || !strings.Contains(filtered.Body.String(), `href="/?view=all"`) {
		t.Fatalf("unwatched = %q", filtered.Body.String())
	}
}
