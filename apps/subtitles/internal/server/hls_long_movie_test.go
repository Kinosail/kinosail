package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionlessHLSKeeps170MinuteMovieWithinManifestBounds(t *testing.T) {
	const header = "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-MAP:URI=\"init.mp4\"\n"
	completed := longMovieHLSManifest(header)
	want := strings.Replace(completed, ":EVENT\n", ":VOD\n", 1)
	for name, manifest := range map[string]string{
		"growing four-second rendition":   header + "#EXTINF:4.000000,\nsegment-00000.m4s\n",
		"completed four-second rendition": completed,
	} {
		t.Run(name, func(t *testing.T) { assertLongMovieHLSResponse(t, manifest, want) })
	}
}

func longMovieHLSManifest(header string) string {
	var manifest strings.Builder
	manifest.WriteString(header)
	for index := range 2550 {
		fmt.Fprintf(&manifest, "#EXTINF:4.000000,\nsegment-%05d.m4s\n", index)
	}
	manifest.WriteString("#EXT-X-ENDLIST\n")
	return manifest.String()
}

func assertLongMovieHLSResponse(t *testing.T, manifest, want string) {
	t.Helper()
	if len(manifest) > maxHLSPlaylistBytes {
		t.Fatal("170-minute fixture exceeds the unchanged input bound")
	}
	path := filepath.Join(t.TempDir(), "index.m3u8")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/item/index.m3u8", nil)
	response := httptest.NewRecorder()
	serveHLSPlaylistWithSession(response, request, file, 170*60)
	if response.Code != http.StatusOK || response.Body.String() != want {
		t.Fatalf("170-minute playlist status=%d bytes=%d; expected exact bounded VOD timeline", response.Code, response.Body.Len())
	}
	if strings.Count(response.Body.String(), "#EXTINF:") != 2550 || response.Body.Len() > maxHLSPlaylistOutputBytes {
		t.Fatal("170-minute playlist has an incorrect segment count or exceeds its output bound")
	}
	if original, err := os.ReadFile(path); err != nil || string(original) != manifest {
		t.Fatalf("serving changed the cache: %v", err)
	}
}
