package server_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"image"
	"image/jpeg"
	"io/fs"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

// This journey uses the real public Owner API, profile authorization, metadata
// store and image delivery. A disposable provider avoids production accounts.
// Canonical TMDB variant selection is covered separately because this custom
// provider must retain its existing image-base behavior.
func TestOwnerBackdropRefreshReplacesDerivedImageAndPreservesItOnFailure(t *testing.T) {
	poster := refreshedBackdropJPEG(t, 780, 1170)
	legacy := refreshedBackdropJPEG(t, 780, 439)
	sharp := refreshedBackdropJPEG(t, 3840, 2160)
	var phase, providerRequests atomic.Int32
	provider := refreshedBackdropProvider(&phase, &providerRequests, poster, legacy, sharp)
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	originalPath, original := writeRefreshedBackdropOriginal(t, mediaDir)
	cacheDir := t.TempDir()
	handler := server.New(server.Config{
		MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: cacheDir, RequireAuth: true,
		Metadata: server.MetadataConfig{URL: provider.URL, ImageURL: provider.URL, Token: "token"},
	})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	item := refreshedBackdropItem(t, handler, owner.Value)
	refreshPath := "/api/v1/items/" + item.ID + "/metadata/refresh"
	if created := webFormCall(t, handler, owner.Value, "/settings/profiles", url.Values{
		"name": {"Viewer"}, "password": {"viewer-password"}, "rating": {"all"}, "libraries": {"all"},
	}); created.Code != http.StatusSeeOther {
		t.Fatalf("create Viewer = %d", created.Code)
	}
	viewer := signInTestProfile(t, handler, "/login", "name=Viewer&password=viewer-password")
	if seed := apiCall(t, handler, owner.Value, http.MethodPost, refreshPath, nil); seed.Code != http.StatusNoContent {
		t.Fatalf("seed artwork = %d", seed.Code)
	}
	item = refreshedBackdropItem(t, handler, owner.Value)
	derivedDir := filepath.Join(cacheDir, "metadata")
	journey := refreshedBackdropJourney{
		handler: handler, owner: owner.Value, item: item, refreshPath: refreshPath, derivedDir: derivedDir,
		originalPath: originalPath, original: original, poster: poster, legacy: legacy,
		phase: &phase, providerRequests: &providerRequests,
	}
	for _, test := range []struct {
		name, token string
		status      int
	}{
		{"unauthenticated", "", http.StatusUnauthorized},
		{"invalid session", "invalid-session", http.StatusUnauthorized},
		{"Viewer", viewer.Value, http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			journey.assertDenied(t, test.token, test.status)
		})
	}
	for _, test := range []struct {
		name   string
		phase  int32
		status int
		image  []byte
		width  int
	}{
		{"legacy seed", 0, http.StatusNoContent, legacy, 780},
		{"sharp replacement", 1, http.StatusNoContent, sharp, 3840},
		{"provider failure", 2, http.StatusBadGateway, sharp, 3840},
		{"oversized response", 3, http.StatusBadGateway, sharp, 3840},
	} {
		t.Run(test.name, func(t *testing.T) {
			journey.assertRefresh(t, test.phase, test.status, test.image, test.width)
		})
	}
}

func writeRefreshedBackdropOriginal(t *testing.T, directory string) (string, []byte) {
	t.Helper()
	path := filepath.Join(directory, "Arrival.mp4")
	data := []byte("opaque original media; artwork refresh must not rewrite it")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, data
}

func refreshedBackdropItem(t *testing.T, handler http.Handler, token string) landscapeLibraryItem {
	t.Helper()
	response := apiCall(t, handler, token, http.MethodGet, "/api/v1/library", nil)
	var page struct{ Items []landscapeLibraryItem }
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &page) != nil || len(page.Items) != 1 {
		t.Fatalf("library = %d; items=%d", response.Code, len(page.Items))
	}
	return page.Items[0]
}

func refreshedBackdropJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	var data bytes.Buffer
	if err := jpeg.Encode(&data, image.NewGray(image.Rect(0, 0, width, height)), &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

type refreshedBackdropCacheFile struct {
	digest   [32]byte
	modified int64
	mode     fs.FileMode
}

func refreshedBackdropCacheSnapshot(t *testing.T, directory string) map[string]refreshedBackdropCacheFile {
	t.Helper()
	files := make(map[string]refreshedBackdropCacheFile)
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			data, err := root.ReadFile(path)
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			files[filepath.Join(directory, path)] = refreshedBackdropCacheFile{sha256.Sum256(data), info.ModTime().UnixNano(), info.Mode()}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

type refreshedBackdropJourney struct {
	handler                                      http.Handler
	owner, refreshPath, derivedDir, originalPath string
	item                                         landscapeLibraryItem
	original, poster, legacy                     []byte
	phase, providerRequests                      *atomic.Int32
}

func (journey refreshedBackdropJourney) assertDenied(t *testing.T, token string, status int) {
	t.Helper()
	before := journey.providerRequests.Load()
	cacheBefore := refreshedBackdropCacheSnapshot(t, journey.derivedDir)
	response := apiCall(t, journey.handler, token, http.MethodPost, journey.refreshPath, nil)
	if response.Code != status || journey.providerRequests.Load() != before {
		t.Fatalf("denied refresh = %d; provider requests changed=%t", response.Code, journey.providerRequests.Load() != before)
	}
	unchanged := refreshedBackdropItem(t, journey.handler, journey.owner)
	if unchanged.Artwork != journey.item.Artwork || unchanged.Backdrop != journey.item.Backdrop {
		t.Fatal("denied refresh changed artwork references")
	}
	if !maps.Equal(cacheBefore, refreshedBackdropCacheSnapshot(t, journey.derivedDir)) {
		t.Fatal("denied refresh wrote cache files")
	}
	journey.assertServedBytes(t, unchanged, journey.legacy)
}

func (journey refreshedBackdropJourney) assertRefresh(t *testing.T, phase int32, status int, image []byte, width int) {
	t.Helper()
	journey.phase.Store(phase)
	response := apiCall(t, journey.handler, journey.owner, http.MethodPost, journey.refreshPath, nil)
	if response.Code != status {
		t.Fatalf("refresh = %d, want %d", response.Code, status)
	}
	current := refreshedBackdropItem(t, journey.handler, journey.owner)
	if current.Artwork != "/art/"+journey.item.ID || current.Backdrop != "/backdrop/"+journey.item.ID {
		t.Fatalf("refresh lost visible artwork references: %#v", current)
	}
	backdrop := journey.assertServedBytes(t, current, image)
	config, err := jpeg.DecodeConfig(bytes.NewReader(backdrop))
	if err != nil || config.Width != width {
		t.Fatalf("served backdrop width = %d, want %d, err=%v", config.Width, width, err)
	}
	media, err := os.ReadFile(journey.originalPath)
	if err != nil || !bytes.Equal(media, journey.original) {
		t.Fatalf("refresh changed original media: %v", err)
	}
}

func (journey refreshedBackdropJourney) assertServedBytes(t *testing.T, item landscapeLibraryItem, want []byte) []byte {
	t.Helper()
	art := apiCall(t, journey.handler, journey.owner, http.MethodGet, item.Artwork, nil)
	backdrop := apiCall(t, journey.handler, journey.owner, http.MethodGet, item.Backdrop, nil)
	if art.Code != http.StatusOK || !bytes.Equal(art.Body.Bytes(), journey.poster) || backdrop.Code != http.StatusOK || !bytes.Equal(backdrop.Body.Bytes(), want) {
		t.Fatalf("derived bytes changed unexpectedly: poster=%d backdrop=%d", art.Code, backdrop.Code)
	}
	return backdrop.Body.Bytes()
}

func refreshedBackdropProvider(phase, requests *atomic.Int32, poster, legacy, sharp []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		serveRefreshedBackdrop(writer, request, phase, poster, legacy, sharp)
	}))
}

func serveRefreshedBackdrop(writer http.ResponseWriter, request *http.Request, phase *atomic.Int32, poster, legacy, sharp []byte) {
	switch request.URL.Path {
	case "/search/movie":
		if request.Header.Get("Authorization") != "Bearer token" {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"results": []any{map[string]any{
			"id": 101, "title": "Arrival", "release_date": "2016-11-11",
			"poster_path": "/poster.jpg", "backdrop_path": "/backdrop.jpg",
		}}})
	case "/poster.jpg":
		writer.Header().Set("Content-Type", "image/jpeg")
		_, _ = writer.Write(poster)
	case "/backdrop.jpg":
		if phase.Load() == 2 {
			http.Error(writer, "temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Content-Type", "image/jpeg")
		switch phase.Load() {
		case 0:
			_, _ = writer.Write(legacy)
		case 1:
			_, _ = writer.Write(sharp)
		case 3:
			_, _ = writer.Write(make([]byte, (8<<20)+1))
		}
	default:
		fakeTMDB(writer, request)
	}
}
