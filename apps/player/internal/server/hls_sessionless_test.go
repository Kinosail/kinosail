package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestSessionlessHLSPlaylist(t *testing.T) {
	servertest.SessionlessHLSPlaylist(t, 2, serveHLSPlaylistWithSession)
}

func TestSessionlessHLSMissingManifestAndInvalidSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.m3u8")
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/item/index.m3u8", nil)
	response := httptest.NewRecorder()
	if !serveHLSPlaylistWithSession(response, request, path, 0, 4) || response.Code != http.StatusNotFound {
		t.Fatalf("missing playlist status = %d", response.Code)
	}
	for _, query := range []string{"short", "bad%20session", "bad%0Asession"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/item/index.m3u8?playSessionId="+query, nil)
		response := httptest.NewRecorder()
		if serveHLSPlaylistWithSession(response, request, path, 0, 4) || response.Body.Len() != 0 {
			t.Fatal("invalid session no longer falls back without reading the manifest")
		}
	}
}
