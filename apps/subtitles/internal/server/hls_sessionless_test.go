package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestSessionlessHLSPlaylist(t *testing.T) {
	servertest.SessionlessHLSPlaylist(t, 4, func(writer http.ResponseWriter, request *http.Request, path string, _ int, duration float64) bool {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		serveHLSPlaylistWithSession(writer, request, file, duration)
		return true
	})
}

func TestSessionlessHLSProjectionRetainsOutputBoundsAndInvalidSessionFallback(t *testing.T) {
	const manifest = "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:4,\nsegment-00000.m4s\n"
	path := filepath.Join(t.TempDir(), "index.m3u8")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/item/index.m3u8", nil)
	serveHLSPlaylistWithSession(response, request, file, 20000)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("oversized projection status = %d", response.Code)
	}
	if original, err := os.ReadFile(path); err != nil || string(original) != manifest {
		t.Fatalf("rejected projection changed cache: %q, %v", original, err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	request.URL.RawQuery = "playSessionId=invalid%20session"
	response = httptest.NewRecorder()
	serveHLSPlaylistWithSession(response, request, file, 20000)
	if response.Code != http.StatusOK || response.Body.String() != manifest {
		t.Fatalf("invalid session fallback changed: %d %q", response.Code, response.Body.String())
	}
}
