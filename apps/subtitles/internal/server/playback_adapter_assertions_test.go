package server_test

import (
	"net/http"
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
