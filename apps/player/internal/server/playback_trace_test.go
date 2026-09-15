package server

import (
	"bytes"
	"encoding/json"
	"html"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestPlaybackTraceCorrelatesBrowserAndMediaWithoutPrivateData(t *testing.T) {
	media := t.TempDir()
	path := filepath.Join(media, "Private Film.mp4")
	if err := os.WriteFile(path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	handler := New(Config{MediaDir: media})
	home := playbackTraceRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	page := playbackTraceRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	session := regexp.MustCompile(`data-playback-session=?"?([a-zA-Z0-9_-]+)`).FindStringSubmatch(page.Body.String())
	source := regexp.MustCompile(`src=?"?(/media/[^" >]+)`).FindStringSubmatch(page.Body.String())
	if page.Code != http.StatusOK || len(session) != 2 || len(source) != 2 || !strings.Contains(source[1], "playbackSession="+session[1]) {
		t.Fatalf("player trace contract is missing: %d %q", page.Code, page.Body.String())
	}

	payload := map[string]any{"session": session[1], "event": "play-rejected", "sequence": 4, "elapsedMs": 812, "positionMs": 42000, "durationMs": 7200000, "bufferedAheadMs": 250, "readyState": 2, "networkState": 2, "paused": true, "method": "direct", "detail": "control:NotAllowedError"}
	body, _ := json.Marshal(payload)
	event := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/items/"+id+"/playback-events", bytes.NewReader(body))
	event.Header.Set("Content-Type", "application/json")
	eventResponse := playbackTraceRequest(handler, event)
	mediaRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, html.UnescapeString(source[1]), nil)
	mediaRequest.Header.Set("Range", "bytes=0-0")
	mediaResponse := playbackTraceRequest(handler, mediaRequest)
	logs := output.String()
	assertPlaybackTraceLogs(t, logs, session[1], eventResponse, mediaResponse)
	for _, private := range []string{path, "Private Film", id, source[1]} {
		if strings.Contains(logs, private) {
			t.Fatalf("playback trace exposed %q: %s", private, logs)
		}
	}
}

func TestPlaybackTraceRejectsInvalidInputWithoutLogging(t *testing.T) {
	fixture := servertest.LibraryAPIFixture{NewHandler: func(media, data string, requireAuth bool) http.Handler {
		return New(Config{MediaDir: media, DataDir: data, RequireAuth: requireAuth})
	}}
	fixture.PlaybackTraceRejectsInvalidInputWithoutLogging(t)
}

func TestDisplayedFramePlaybackTraceEventsAreAccepted(t *testing.T) {
	t.Parallel()
	for _, event := range []string{"capability-direct", "first-frame", "frame-after-seek"} {
		if !playback.ValidTrace(playbackTraceEvent{Session: "trace-session", Event: event, Sequence: 1}, validPlaybackSession) {
			t.Fatalf("%s trace was rejected", event)
		}
	}
}

func playbackTraceRequest(handler http.Handler, request *http.Request) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertPlaybackTraceLogs(t *testing.T, logs, session string, eventResponse, mediaResponse *httptest.ResponseRecorder) {
	t.Helper()
	if eventResponse.Code != http.StatusNoContent || mediaResponse.Code != http.StatusPartialContent || strings.Count(logs, `"playback_session":"`+session+`"`) < 3 {
		t.Fatalf("trace response=%d media=%d logs=%s", eventResponse.Code, mediaResponse.Code, logs)
	}
	for _, expected := range []string{`"msg":"playback planned"`, `"msg":"playback trace"`, `"event":"play-rejected"`, `"detail":"control:NotAllowedError"`, `"method":"direct"`, `"direct_type":`, `"request_range":"bytes=0-0"`, `"response_range":"bytes 0-0/5"`, `"response_bytes":1`} {
		if !strings.Contains(logs, expected) {
			t.Fatalf("trace response=%d media=%d logs=%s", eventResponse.Code, mediaResponse.Code, logs)
		}
	}
}
