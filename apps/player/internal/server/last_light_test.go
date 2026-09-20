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

func TestLastLightHomeUsesRealLibraryArtwork(t *testing.T) {
	media := t.TempDir()
	for name, data := range map[string]string{"Arrival (2016).mp4": "video", "Arrival (2016).jpg": "poster", "Arrival (2016)-fanart.jpg": "backdrop"} {
		if err := os.WriteFile(filepath.Join(media, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: media})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK {
		t.Fatalf("home: %d %s", response.Code, body)
	}
	for _, fragment := range []string{`class="home-feature"`, `class="watch-progress"`, `aria-label="Watch progress"`, `data-feature-title="Arrival"`, `class="app-header"`, `Recently added`, `/static/app.css?v=electric-22`} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("home missing %s", fragment)
		}
	}
	match := regexp.MustCompile(`class="home-feature"[^>]* data-palette-id="([a-f0-9]+)"`).FindStringSubmatch(body)
	if len(match) != 2 {
		t.Fatalf("feature did not select an indexed title: %s", regexp.MustCompile(`<section class="home-feature"[^>]*>`).FindString(body))
	}
	filtered := httptest.NewRecorder()
	handler.ServeHTTP(filtered, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=movies", nil))
	if strings.Contains(filtered.Body.String(), `class="home-feature"`) {
		t.Fatal("browse page must retain its focused library layout")
	}
}

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
