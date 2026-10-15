package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestConfiguredProviderAutomaticallyEnrichesLibrary(t *testing.T) {
	t.Parallel()
	provider := httptest.NewServer(http.HandlerFunc(fakeTMDB))
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: t.TempDir(), Metadata: server.MetadataConfig{URL: provider.URL, ImageURL: provider.URL, Token: "token"}})
	firstAPIItemID(t, handler, "")
	for range 30 {
		player := httptest.NewRecorder()
		handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
		if strings.Contains(player.Body.String(), "A linguist meets visitors.") {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("automatic metadata did not appear")
}

func TestConfiguredProviderAutomaticallyEnrichesMediaAddedLater(t *testing.T) {
	t.Parallel()
	provider := httptest.NewServer(http.HandlerFunc(fakeTMDB))
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: t.TempDir(), Metadata: server.MetadataConfig{URL: provider.URL, ImageURL: provider.URL, Token: "token"}})
	time.Sleep(350 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(mediaDir, "Dune.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	scan := httptest.NewRecorder()
	handler.ServeHTTP(scan, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/tasks/scan", nil))
	library := httptest.NewRecorder()
	handler.ServeHTTP(library, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
	var catalog struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(library.Body.Bytes(), &catalog); err != nil || len(catalog.Items) != 1 {
		t.Fatalf("library = %d %q err=%v", library.Code, library.Body.String(), err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		player := httptest.NewRecorder()
		handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
		if strings.Contains(player.Body.String(), "A linguist meets visitors.") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("metadata for media added after startup was not maintained automatically")
}
