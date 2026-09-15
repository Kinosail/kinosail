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

func TestPlayerShowsMediaInformation(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Heat.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))

	if !strings.Contains(player.Body.String(), "Media information") || !strings.Contains(player.Body.String(), "MP4") || !strings.Contains(player.Body.String(), "5 B") {
		t.Fatalf("player = %q", player.Body.String())
	}
}
