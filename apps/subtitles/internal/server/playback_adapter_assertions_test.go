package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func assertCapabilityPlaybackWebAndAPI(t *testing.T, handler http.Handler, id, token string, owner *http.Cookie) {
	t.Helper()
	rawID := id[:16]
	api := apiCall(t, handler, token, http.MethodGet, "/api/v1/items/"+rawID+"/playback", nil)
	if api.Code != http.StatusOK || !strings.Contains(api.Body.String(), `"mode":"direct"`) || !strings.Contains(api.Body.String(), `"reason":"direct-preferred"`) || !strings.Contains(api.Body.String(), `"compatiblePlan":{"allowed":true,"mode":"transcode"`) || !strings.Contains(api.Body.String(), `"compatible":"/hls/`) {
		t.Fatalf("API playback = %d %q", api.Code, api.Body.String())
	}
	page := requestWithCookie(t, handler, http.MethodGet, "/watch/"+rawID, "", owner)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `src="/media/`+rawID+`?playbackSession=`) || !strings.Contains(page.Body.String(), `data-adaptive="/hls/`) || strings.Contains(page.Body.String(), `data-hls=`) {
		t.Fatalf("web playback = %d %q", page.Code, page.Body.String())
	}
}

func assertRejectedJellyfinTranscodeChildren(t *testing.T, handler http.Handler, id, token, cache string) {
	t.Helper()
	before, err := os.ReadDir(cache)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/Videos/" + id + "/360p/index.m3u8",
		"/Videos/" + id + "/p/invalid/360p/index.m3u8",
		"/Videos/" + id + "/p/" + strings.Repeat("x", 257) + "/360p/index.m3u8",
	} {
		if response := jellyfinCall(t, handler, http.MethodGet, path, "", token); response.Code != http.StatusNotFound {
			t.Fatalf("invalid transcode child %q = %d %q", path, response.Code, response.Body.String())
		}
	}
	after, err := os.ReadDir(cache)
	if err != nil || len(after) != len(before) {
		t.Fatalf("invalid transcode children changed cache: before=%d after=%d err=%v", len(before), len(after), err)
	}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/Videos/"+id+"/master.m3u8?playSessionId=missing", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("invalid play session = %d %q", response.Code, response.Body.String())
	}
}
