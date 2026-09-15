package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEventPlaylistStartsAtItsFirstReadySegment(t *testing.T) {
	manifest := []byte("#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:1,\nsegment-00000.m4s\n")
	rewritten, err := hlsPlaylistWithSession(manifest, "playback-session")
	if err != nil {
		t.Fatal(err)
	}
	result := string(rewritten)
	if !strings.Contains(result, "#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-START:TIME-OFFSET=0,PRECISE=YES\n") {
		t.Fatalf("event playlist start = %q", result)
	}
}

func TestSessionPlaylistRejectsUnsafeOrExcessiveMediaReferences(t *testing.T) {
	for name, manifest := range map[string][]byte{
		"absolute URL":         []byte("#EXTM3U\nhttps://outside.example/segment.m4s\n"),
		"tag URL":              []byte("#EXTM3U\n#EXT-X-MAP:URI=\"https://outside.example/init.mp4\"\n"),
		"unquoted tag URL":     []byte("#EXTM3U\n#EXT-X-MAP:URI=https://outside.example/init.mp4\n"),
		"spaced tag URL":       []byte("#EXTM3U\n#EXT-X-MAP:URI = \"https://outside.example/init.mp4\"\n"),
		"tag trailing junk":    []byte("#EXTM3U\n#EXT-X-MAP:URI=\"init.mp4\"junk\n"),
		"invalid segment":      []byte("#EXTM3U\nsegment-x/../.m4s\n"),
		"too many media":       []byte("#EXTM3U\n" + strings.Repeat("segment-00000.m4s\n", maxHLSPlaylistURIs+1)),
		"too many blank lines": []byte(strings.Repeat("\n", maxHLSPlaylistLines+1)),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "index.m3u8")
			if err := os.WriteFile(path, manifest, 0o600); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = file.Close() })
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/index.m3u8?playSessionId=playback-session", nil)
			response := httptest.NewRecorder()
			serveHLSPlaylistWithSession(response, request, file, 0)
			if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), "outside.example") {
				t.Fatalf("unsafe session playlist = %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestSessionlessPlaylistStillRejectsUnsafeMediaReference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.m3u8")
	if err := os.WriteFile(path, []byte("#EXTM3U\nhttps://outside.example/segment.m4s\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/index.m3u8", nil)
	serveHLSPlaylistWithSession(response, request, file, 0)
	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), "outside.example") {
		t.Fatalf("unsafe sessionless playlist = %d %q", response.Code, response.Body.String())
	}
}

func TestSessionPlaylistRejectsOversizedManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.m3u8")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", maxHLSPlaylistBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/index.m3u8?playSessionId=playback-session", nil)
	response := httptest.NewRecorder()
	serveHLSPlaylistWithSession(response, request, file, 0)
	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), strings.Repeat("x", 100)) {
		t.Fatalf("oversized session playlist = %d %q", response.Code, response.Body.String())
	}
}
