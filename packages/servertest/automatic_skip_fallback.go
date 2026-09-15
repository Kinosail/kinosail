package servertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func (fixture AutomaticSkipFixture) AutomaticPlaybackKeepsDirectTimelineAndOffersCompatibleFallback(t *testing.T) {
	t.Helper()
	handler, apiToken, jellyfinToken, rawID, jellyfinID, _ := fixture.server(t)

	fallback := APICall(t, handler, apiToken, http.MethodGet, "/api/v1/items/"+rawID+"/playback", nil)
	AssertAPIBody(t, fallback, http.StatusOK, `"mode":"direct"`, `"reason":"direct-preferred"`, `"duration":60`, `"type":"intro"`, `"autoSkip":["intro"]`, `"compatible":"/hls/`)
	unsupported := jellyfinPlaybackRequest(t, handler, jellyfinToken, jellyfinID, "Swiftfin", "1.6")
	AssertAPIBody(t, unsupported, http.StatusOK, `"SupportsDirectPlay":false`, `"SupportsDirectStream":true`, `"SupportsTranscoding":true`, `"RunTimeTicks":500000000`, `"TranscodingUrl":"/Videos/`)
	supported := jellyfinPlaybackRequest(t, handler, jellyfinToken, jellyfinID, "Jellyfin for Android", "2.7.0")
	AssertAPIBody(t, supported, http.StatusOK, `"SupportsDirectPlay":false`, `"SupportsDirectStream":true`, `"SupportsTranscoding":true`, `"RunTimeTicks":500000000`, `"TranscodingUrl":"/Videos/`)
	directFallback := AutomaticSkipJellyfinRequest(t, handler, jellyfinToken, http.MethodGet, "/Videos/"+jellyfinID+"/stream", "Swiftfin", "1.6", "")
	if directFallback.Code != http.StatusOK || !strings.Contains(directFallback.Body.String(), "#EXT-X-STREAM-INF:") {
		t.Fatalf("unsupported direct stream = %d %q", directFallback.Code, directFallback.Body.String())
	}
	directSupported := AutomaticSkipJellyfinRequest(t, handler, jellyfinToken, http.MethodGet, "/Videos/"+jellyfinID+"/stream", "Jellyfin for Android", "2.7.0", "")
	if directSupported.Code != http.StatusOK || !strings.Contains(directSupported.Body.String(), "#EXT-X-STREAM-INF:") {
		t.Fatalf("supported direct stream = %d %q", directSupported.Code, directSupported.Body.String())
	}

	segments := AutomaticSkipJellyfinRequest(t, handler, jellyfinToken, http.MethodGet, "/MediaSegments/"+jellyfinID, "Jellyfin for Android", "2.7.0", "")
	AssertAPIBody(t, segments, http.StatusOK, `"Items":[]`, `"TotalRecordCount":0`)
	unsupportedSegments := AutomaticSkipJellyfinRequest(t, handler, jellyfinToken, http.MethodGet, "/MediaSegments/"+jellyfinID, "Swiftfin", "1.6", "")
	AssertAPIBody(t, unsupportedSegments, http.StatusOK, `"Items":[]`, `"TotalRecordCount":0`)
	disabled := APICall(t, handler, apiToken, http.MethodPut, "/api/v1/settings/playback", map[string]any{"mode": "automatic", "autoplay": true, "subtitles": "on", "autoSkip": []string{}})
	AssertAPIBody(t, disabled, http.StatusOK, `"status":"saved"`)
	original := APICall(t, handler, apiToken, http.MethodGet, "/api/v1/items/"+rawID+"/playback", nil)
	AssertAPIBody(t, original, http.StatusOK, `"mode":"direct"`, `"duration":60`, `"direct":"/media/`, `"type":"intro"`)
	visibleSegments := AutomaticSkipJellyfinRequest(t, handler, jellyfinToken, http.MethodGet, "/MediaSegments/"+jellyfinID, "Jellyfin for Android", "2.7.0", "")
	AssertAPIBody(t, visibleSegments, http.StatusOK, `"Type":"Intro"`, `"StartTicks":100000000`, `"EndTicks":200000000`)
}

func (fixture AutomaticSkipFixture) ServerAutomaticSkipRenditionRemovesConfiguredRanges(t *testing.T) {
	t.Helper()
	handler, apiToken, _, rawID, _, arguments := fixture.server(t)
	setCompatiblePlayback(t, handler, apiToken)
	playback := APICall(t, handler, apiToken, http.MethodGet, "/api/v1/items/"+rawID+"/playback", nil)
	var result struct {
		Compatible string `json:"compatible"`
	}
	MustJSON(t, playback, &result)
	playlist := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, result.Compatible, nil)
	request.Header.Set("Authorization", "Bearer "+apiToken)
	handler.ServeHTTP(playlist, request)
	generic := httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+rawID+"/index.m3u8", nil)
	request.Header.Set("Authorization", "Bearer "+apiToken)
	handler.ServeHTTP(generic, request)
	used, err := os.ReadFile(arguments)
	if err != nil || playlist.Code != http.StatusOK || generic.Code != http.StatusOK {
		t.Fatalf("fallback playlists = %d %q and %d %q, arguments = %q, error = %v", playlist.Code, playlist.Body.String(), generic.Code, generic.Body.String(), used, err)
	}
	commands := strings.Split(strings.TrimSpace(string(used)), "\n")
	if len(commands) != 1 {
		t.Fatalf("FFmpeg commands = %q", used)
	}
	for _, command := range commands {
		for _, expected := range []string{"aselect=", "10", "20"} {
			if !strings.Contains(command, expected) {
				t.Fatalf("automatic-skip FFmpeg arguments lack %q: %q", expected, command)
			}
		}
	}
	if !strings.Contains(string(used), "-f concat") || !strings.Contains(string(used), "-c:v copy") {
		t.Fatalf("automatic-skip rendition does not preserve compatible video: %q", used)
	}
}

func (fixture AutomaticSkipFixture) AutomaticSkipUsesExactTranscodeAwayFromRandomAccessPoints(t *testing.T) {
	t.Helper()
	frames := `[{"key_frame":1,"best_effort_timestamp_time":"0"},{"key_frame":1,"best_effort_timestamp_time":"8"},{"key_frame":1,"best_effort_timestamp_time":"20"},{"key_frame":1,"best_effort_timestamp_time":"60"}]`
	handler, token, _, id, _, arguments := fixture.serverWithFrames(t, frames)
	setCompatiblePlayback(t, handler, token)
	playback := APICall(t, handler, token, http.MethodGet, "/api/v1/items/"+id+"/playback", nil)
	AssertAPIBody(t, playback, http.StatusOK, `"mode":"transcode"`, `"reason":"automatic-marker-skip"`, `"duration":50`)
	var result struct {
		Compatible string `json:"compatible"`
	}
	MustJSON(t, playback, &result)
	playlist := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, result.Compatible, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(playlist, request)
	used, err := os.ReadFile(arguments)
	if err != nil || playlist.Code != http.StatusOK || !strings.Contains(string(used), "-vf select=") || strings.Contains(string(used), "-f concat") {
		t.Fatalf("exact fallback = %d %q, arguments = %q, error = %v", playlist.Code, playlist.Body.String(), used, err)
	}
}
