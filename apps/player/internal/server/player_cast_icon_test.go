package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestPlayerUsesCastIconForDevicePlayback(t *testing.T) {
	t.Parallel()
	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Movie.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))

	body := player.Body.String()
	if !strings.Contains(body, `data-tv-open aria-haspopup="dialog"><i class=i-cast aria-hidden=true></i> Play on TV`) {
		t.Fatalf("player is missing the cast icon control: %q", body)
	}
	if strings.Contains(body, `data-cast>Play on device</button>`) {
		t.Fatal("device playback control should use the cast icon")
	}
}
