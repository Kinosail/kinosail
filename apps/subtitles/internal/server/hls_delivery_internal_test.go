package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func TestHLSDeliveryRejectsCacheKeySymlinkEscape(t *testing.T) {
	cache := t.TempDir()
	outside := t.TempDir()
	item := library.Item{ID: "0123456789abcdef"}
	recipe := hlsRecipe{mode: "remux"}
	key := hlsRecipeKey(item.ID, recipe)
	if err := os.WriteFile(filepath.Join(outside, "segment-00000.m4s"), []byte("outside-cache"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(cache, key)); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/item/segment-00000.m4s", nil)
	(&hlsManager{cache: cache}).serveRecipe(response, request, item, recipe, "segment-00000.m4s")
	if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "outside-cache") {
		t.Fatalf("symlinked cache delivery = %d %q", response.Code, response.Body.String())
	}
}

func TestHLSPlaylistReaderRejectsIntermediateDirectoryEscape(t *testing.T) {
	cache := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "index.m3u8"), []byte("#EXTM3U\noutside-cache\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(cache, "recipe-key")); err != nil {
		t.Fatal(err)
	}
	if data, err := playback.ReadHLSPlaylist(filepath.Join(cache, "recipe-key", "index.m3u8")); err == nil || strings.Contains(string(data), "outside-cache") {
		t.Fatalf("intermediate cache symlink was read: %q, err=%v", data, err)
	}
}
