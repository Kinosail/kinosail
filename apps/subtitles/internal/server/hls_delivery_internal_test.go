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

func TestHLSPlaylistReaderRejectsSymlinkAndOversizedCacheFiles(t *testing.T) {
	directory := t.TempDir()
	external := filepath.Join(t.TempDir(), "outside.m3u8")
	if err := os.WriteFile(external, []byte("#EXTM3U\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(directory, "linked.m3u8")
	if err := os.Symlink(external, linked); err != nil {
		t.Fatal(err)
	}
	if _, err := playback.ReadHLSPlaylist(linked); err == nil {
		t.Fatal("HLS playlist reader followed a symlink")
	}
	oversized := filepath.Join(directory, "oversized.m3u8")
	if err := os.WriteFile(oversized, []byte(strings.Repeat("x", maxHLSPlaylistBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := playback.ReadHLSPlaylist(oversized); err == nil {
		t.Fatal("HLS playlist reader accepted an oversized cache file")
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
