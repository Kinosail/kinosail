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

func TestViewerCanSeeSidecarArtwork(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	for name, content := range map[string]string{"Alien.mp4": "video", "Alien.jpg": "poster"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	homeResponse := httptest.NewRecorder()
	handler.ServeHTTP(homeResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	match := regexp.MustCompile(`/art/([a-f0-9]+)`).FindStringSubmatch(homeResponse.Body.String())
	if len(match) != 2 {
		t.Fatalf("home has no artwork: %q", homeResponse.Body.String())
	}
	artResponse := httptest.NewRecorder()
	handler.ServeHTTP(artResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/art/"+match[1], nil))
	if artResponse.Body.String() != "poster" {
		t.Fatalf("art = %q", artResponse.Body.String())
	}
}

func TestResponsesCarryBrowserSecurityPolicy(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	server.New(server.Config{}).ServeHTTP(response, request)

	if !strings.Contains(response.Header().Get("Content-Security-Policy"), "default-src 'self'") || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("security headers = %v", response.Header())
	}
}

func TestViewerCanBrowseScannedLibrary(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "The.Matrix.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	server.New(server.Config{MediaDir: mediaDir}).ServeHTTP(response, request)

	if !strings.Contains(response.Body.String(), "The Matrix") {
		t.Fatalf("home = %q", response.Body.String())
	}
}

func TestViewerCanSearchLibrary(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	for _, name := range []string{"Arrival.mp4", "The.Matrix.mp4"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?q=arrival", nil)
	response := httptest.NewRecorder()
	server.New(server.Config{MediaDir: mediaDir}).ServeHTTP(response, request)

	body := response.Body.String()
	if !strings.Contains(body, "Arrival") || strings.Contains(body, "The Matrix") {
		t.Fatalf("search = %q", body)
	}
}

func TestOwnerCanManuallySyncLibrary(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	handler := server.New(server.Config{MediaDir: mediaDir})
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
	if err := os.WriteFile(filepath.Join(mediaDir, "Dune.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/scan", nil))
	refreshed := httptest.NewRecorder()
	handler.ServeHTTP(refreshed, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))

	if response.Code != http.StatusSeeOther || !strings.Contains(refreshed.Body.String(), "Dune") {
		t.Fatalf("sync = %d, refreshed = %q", response.Code, refreshed.Body.String())
	}
}
