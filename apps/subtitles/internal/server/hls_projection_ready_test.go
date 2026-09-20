package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

const projectionBootstrap = "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:0.041667,\nsegment-00000.m4s\n#EXTINF:0.041667,\nsegment-00001.m4s\n"

func TestHLSDeliveryWaitsForAtomicProjectionReadinessBeforeCompletion(t *testing.T) {
	manager, item, recipe, path := projectionHLSFixture(t, projectionBootstrap)
	ready := projectionBootstrap + "#EXTINF:1.916667,\nsegment-00002.m4s\n#EXTINF:4,\nsegment-00003.m4s\n"
	response := httptest.NewRecorder()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/hls/item/360p/index.m3u8", nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		manager.serveRecipe(response, request, item, recipe, "360p/index.m3u8")
	}()
	select {
	case <-done:
		t.Fatal("bootstrap was served before its cadence was established")
	case <-time.After(50 * time.Millisecond):
	}
	if err := writeAtomicFile(path, []byte(ready)); err != nil {
		cancel()
		<-done
		t.Fatal(err)
	}
	<-done
	if response.Code != http.StatusOK || strings.Count(response.Body.String(), "#EXTINF:") != 5 || !strings.Contains(response.Body.String(), "#EXT-X-ENDLIST") {
		t.Fatalf("ready playlist = %d %q", response.Code, response.Body.String())
	}
	if cached, err := os.ReadFile(path); err != nil || string(cached) != ready {
		t.Fatalf("projection changed the incomplete encoder manifest: %q, %v", cached, err)
	}
}

func TestHLSDeliveryProjectionRejectsUnreadyAndUnsafeManifests(t *testing.T) {
	for name, manifest := range map[string]string{
		"deadline":   projectionBootstrap,
		"unsafe URI": "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:4,\n../outside.m4s\n",
		"oversized":  strings.Repeat("#", maxHLSPlaylistBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			manager, item, recipe, path := projectionHLSFixture(t, manifest)
			ctx, cancel := context.WithTimeout(t.Context(), 75*time.Millisecond)
			defer cancel()
			request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/hls/item/360p/index.m3u8", nil)
			response := httptest.NewRecorder()
			manager.serveRecipe(response, request, item, recipe, "360p/index.m3u8")
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("rejected playlist status = %d", response.Code)
			}
			if cached, err := os.ReadFile(path); err != nil || string(cached) != manifest {
				t.Fatalf("rejected playlist changed cache: %v", err)
			}
		})
	}
}

func projectionHLSFixture(t *testing.T, manifest string) (*hlsManager, library.Item, hlsRecipe, string) {
	t.Helper()
	manager, item, recipe, directory := readyHLSFixture(t)
	manager.probe.inspect(t.Context(), item)
	manager.settings = newSettingsStore(filepath.Dir(item.Path), "", "", nil)
	options, err := manager.settings.transcodingFor(recipe.codec)
	if err != nil {
		t.Fatal(err)
	}
	identity := options.Cache + ":" + sourceVersion(item.Path) + ":" + recipe.token() + ":hls=7"
	master := "#EXTM3U\n#KINOSAIL-TRANSCODER:" + identity + "\n#EXT-X-STREAM-INF:BANDWIDTH=1\n360p/index.m3u8\n"
	if err := writeAtomicFile(filepath.Join(filepath.Dir(directory), "index.m3u8"), []byte(master)); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "index.m3u8")
	if err := writeAtomicFile(path, []byte(manifest)); err != nil {
		t.Fatal(err)
	}
	return manager, item, recipe, path
}
