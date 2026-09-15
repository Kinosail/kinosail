package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestHLSDeliveryWaitsOnlyForPublishedActiveSegments(t *testing.T) {
	manager, item, recipe, directory := readyHLSFixture(t)
	result := make(chan error, 1)
	go func() {
		time.Sleep(25 * time.Millisecond)
		result <- os.WriteFile(filepath.Join(directory, "segment-00001.m4s"), []byte("ready"), 0o600)
	}()
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/item/360p/segment-00001.m4s", nil)
	manager.serveRecipe(response, request, item, recipe, "360p/segment-00001.m4s")
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || response.Body.String() != "ready" {
		t.Fatalf("future segment = %d %q", response.Code, response.Body.String())
	}
}

func TestHLSDeliveryUnknownInactiveAndCanceledSegmentsStayBounded(t *testing.T) {
	manager, item, recipe, directory := readyHLSFixture(t)
	for _, name := range []string{"360p/segment-99999.m4s", "360p/missing.mp4"} {
		started := time.Now()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/item/"+name, nil)
		response := httptest.NewRecorder()
		manager.serveRecipe(response, request, item, recipe, name)
		if response.Code != http.StatusNotFound || time.Since(started) > time.Second {
			t.Fatalf("unknown segment status = %d", response.Code)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/hls/item/360p/segment-00001.m4s", nil)
	response := httptest.NewRecorder()
	manager.serveRecipe(response, request, item, recipe, "360p/segment-00001.m4s")
	if response.Code != http.StatusNotFound {
		t.Fatalf("canceled segment status = %d", response.Code)
	}
	manager.jobs = nil
	response = httptest.NewRecorder()
	manager.serveRecipe(response, request.WithContext(t.Context()), item, recipe, "360p/segment-00001.m4s")
	if response.Code != http.StatusNotFound {
		t.Fatalf("inactive segment status = %d", response.Code)
	}
	if entries, err := os.ReadDir(directory); err != nil || len(entries) != 1 {
		t.Fatalf("rejected segment requests changed cache: %v, %v", entries, err)
	}
}

func TestInvalidPlaybackSessionUsesOnlyPublishedManifestSegments(t *testing.T) {
	manager, item, recipe, directory := readyHLSFixture(t)
	root, err := os.OpenRoot(manager.cache)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/item/360p/index.m3u8?playSessionId=invalid%20session", nil)
	key := hlsRecipeKey(item.ID, recipe)
	for _, test := range []struct {
		name      string
		published bool
	}{
		{"360p/segment-00000.m4s", true},
		{"360p/segment-00001.m4s", false},
	} {
		if published := manager.hlsSegmentPublished(request, root, item, recipe, key, test.name); published != test.published {
			t.Fatalf("published %q = %t, want %t", test.name, published, test.published)
		}
	}
	if manifest, err := os.ReadFile(filepath.Join(directory, "index.m3u8")); err != nil || string(manifest) != "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:4,\nsegment-00000.m4s\n" {
		t.Fatalf("publication check changed manifest: %q, %v", manifest, err)
	}
}

func readyHLSFixture(t *testing.T) (*hlsManager, library.Item, hlsRecipe, string) {
	t.Helper()
	cache := t.TempDir()
	source := filepath.Join(t.TempDir(), "source.mp4")
	if err := os.WriteFile(source, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(t.TempDir(), "ffprobe")
	servertest.WriteExecutable(t, probe, "#!/bin/sh\nprintf '%s' '{\"format\":{\"duration\":\"8\"}}'\n")
	item, recipe := library.Item{ID: "0123456789abcdef", Path: source}, hlsRecipe{mode: "remux"}
	key := hlsRecipeKey(item.ID, recipe)
	directory := filepath.Join(cache, key, "360p")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "index.m3u8"), []byte("#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:4,\nsegment-00000.m4s\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return &hlsManager{cache: cache, probe: newMediaProbe(probe), jobs: map[string]*hlsJob{key: {done: make(chan struct{})}}}, item, recipe, directory
}
