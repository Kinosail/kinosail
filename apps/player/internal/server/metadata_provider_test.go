package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestMetadataProviderContract(t *testing.T) {
	servertest.RunMetadataProvider(t, func(t *testing.T, mediaDir, providerURL string) http.Handler {
		return server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: t.TempDir(), Metadata: server.MetadataConfig{URL: providerURL, ImageURL: providerURL, Token: "token"}})
	}, idFirstTMDB)
}

var fakeTMDB = servertest.MetadataProviderFixture

func TestProviderServesUncroppedLandscapeArtwork(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(landscapeProvider))
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: t.TempDir(), Metadata: server.MetadataConfig{URL: provider.URL, ImageURL: provider.URL, Token: "token"}})
	id := readLandscapeItem(t, handler).ID
	refresh := httptest.NewRecorder()
	handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/metadata/"+id+"/refresh", nil))
	if refresh.Code != http.StatusSeeOther {
		t.Fatalf("metadata refresh = %d %q", refresh.Code, refresh.Body.String())
	}
	item := readLandscapeItem(t, handler)
	if item.Artwork != "/art/"+id || item.Backdrop != "/backdrop/"+id {
		t.Fatalf("landscape projection = %#v", item)
	}
	image := httptest.NewRecorder()
	handler.ServeHTTP(image, httptest.NewRequestWithContext(t.Context(), http.MethodGet, item.Backdrop, nil))
	if image.Code != http.StatusOK || image.Body.String() != "backdrop" {
		t.Fatalf("landscape image = %d %q", image.Code, image.Body.String())
	}
}

type landscapeLibraryItem struct{ ID, Artwork, Backdrop string }

func readLandscapeItem(t *testing.T, handler http.Handler) landscapeLibraryItem {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
	var page struct{ Items []landscapeLibraryItem }
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &page) != nil || len(page.Items) != 1 {
		t.Fatalf("library = %d %q", response.Code, response.Body.String())
	}
	return page.Items[0]
}

func landscapeProvider(writer http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case "/search/movie":
		if request.Header.Get("Authorization") != "Bearer token" {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"results": []any{map[string]any{"id": 101, "title": "Arrival", "overview": "A linguist meets visitors.", "release_date": "2016-11-11", "poster_path": "/poster.jpg", "backdrop_path": "/backdrop.jpg"}}})
	case "/backdrop.jpg":
		writer.Header().Set("Content-Type", "image/jpeg")
		_, _ = writer.Write([]byte("backdrop"))
	default:
		fakeTMDB(writer, request)
	}
}
