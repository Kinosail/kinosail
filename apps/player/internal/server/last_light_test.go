package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestLastLightEmptyHomeHasNoInventedFeature(t *testing.T) {
	handler := server.New(server.Config{MediaDir: t.TempDir()})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `class="home-feature"`) {
		t.Fatalf("empty home: %d %s", response.Code, response.Body.String())
	}
}

func TestHomeWithoutArtworkStartsWithTheLibrary(t *testing.T) {
	t.Parallel()
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Example.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: media})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "Recently added") || strings.Contains(body, `class="home-feature"`) {
		t.Fatalf("home must expose library content without an empty banner: %d", response.Code)
	}
}
