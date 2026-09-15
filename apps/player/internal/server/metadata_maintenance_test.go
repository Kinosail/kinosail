package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
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
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	for range 30 {
		player := httptest.NewRecorder()
		handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
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
	time.Sleep(350 * time.Millisecond) // Let initial enrichment finish before adding the new item.
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
		handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+catalog.Items[0].ID, nil))
		if strings.Contains(player.Body.String(), "A linguist meets visitors.") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("metadata for media added after startup was not maintained automatically")
}

func TestProviderGroupsMoviesIntoBoxSets(t *testing.T) {
	t.Parallel()
	provider := httptest.NewServer(http.HandlerFunc(fakeTMDB))
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: t.TempDir(), Metadata: server.MetadataConfig{URL: provider.URL, ImageURL: provider.URL, Token: "token"}})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/metadata/"+id+"/refresh", nil))
	collection := httptest.NewRecorder()
	handler.ServeHTTP(collection, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/collection/Arrival%20Collection", nil))
	if collection.Code != http.StatusOK || !strings.Contains(collection.Body.String(), "Arrival") {
		t.Fatalf("collection = %d %q", collection.Code, collection.Body.String())
	}
	remove := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/collection/Arrival%20Collection/"+id, strings.NewReader("included=false"))
	remove.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), remove)
	collection = httptest.NewRecorder()
	handler.ServeHTTP(collection, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/collection/Arrival%20Collection", nil))
	if strings.Contains(collection.Body.String(), `<h2>Arrival</h2>`) {
		t.Fatalf("removed movie remains in collection: %q", collection.Body.String())
	}
}

func TestOwnerMetadataEditOverridesLocalNFOAndPersists(t *testing.T) {
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.nfo"), []byte("<movie><title>Local title</title><plot>Local plot</plot></movie>"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, CacheDir: t.TempDir()})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/metadata/"+id, strings.NewReader("title=Owner+title&year=2017&plot=Owner+plot"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	player := httptest.NewRecorder()
	server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, CacheDir: t.TempDir()}).ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))

	if response.Code != http.StatusSeeOther || !strings.Contains(player.Body.String(), "Owner title") || !strings.Contains(player.Body.String(), "Owner plot") || !strings.Contains(player.Body.String(), "2017") || strings.Contains(player.Body.String(), "Local title") {
		t.Fatalf("save = %d %q, player = %q", response.Code, response.Body.String(), player.Body.String())
	}
}
