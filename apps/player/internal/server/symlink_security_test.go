package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestServingRejectsMediaReplacedBySymlink(t *testing.T) {
	t.Parallel()
	media, outside := t.TempDir(), filepath.Join(t.TempDir(), "Secret.mp4")
	path := filepath.Join(media, "Movie.mp4")
	if err := os.WriteFile(path, []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: media})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/media/"+id, nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("symlinked media = %d %q", response.Code, response.Body.String())
	}
}
