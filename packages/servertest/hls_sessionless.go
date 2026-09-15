package servertest

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SessionlessHLSPlaylist exercises the actual app playlist-serving operation.
func SessionlessHLSPlaylist(t *testing.T, cadence float64, serve func(http.ResponseWriter, *http.Request, string, int, float64) bool) {
	t.Helper()
	event := fmt.Sprintf("#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:%.6f,\nsegment-00000.m4s\n", cadence)
	vod := strings.Replace(event, ":EVENT", ":VOD", 1)
	for _, test := range []struct {
		name, manifest, want string
		duration             float64
	}{
		{"known duration publishes complete timeline", event, vod + fmt.Sprintf("#EXTINF:%.6f,\nsegment-00001.m4s\n#EXT-X-ENDLIST\n", cadence), 2 * cadence},
		{"completed timeline is preserved", event + "#EXT-X-DISCONTINUITY\n#EXTINF:1.2,\nsegment-00001.m4s\n#EXT-X-ENDLIST\n", vod + "#EXT-X-DISCONTINUITY\n#EXTINF:1.2,\nsegment-00001.m4s\n#EXT-X-ENDLIST\n", 4},
		{"unknown duration remains event", event, event, 0},
		{"negative duration remains event", event, event, -1},
		{"nonfinite duration remains event", event, event, math.NaN()},
		{"oversized duration remains event", event, event, 7*24*60*60 + 1},
		{"missing segments remains event", "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n", "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n", 4},
		{"invalid cadence remains event", strings.ReplaceAll(event, fmt.Sprintf("%.6f", cadence), "NaN"), strings.ReplaceAll(event, fmt.Sprintf("%.6f", cadence), "NaN"), 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertSessionlessHLSTimeline(t, serve, test.manifest, test.want, test.duration)
		})
	}
}

func assertSessionlessHLSTimeline(t *testing.T, serve func(http.ResponseWriter, *http.Request, string, int, float64) bool, manifest, want string, duration float64) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "index.m3u8")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/item/index.m3u8", nil)
	response := httptest.NewRecorder()
	if !serve(response, request, path, 0, duration) {
		t.Fatal("sessionless playlist bypassed manifest handling")
	}
	if response.Code != http.StatusOK || response.Body.String() != want {
		t.Fatalf("playlist status/body = %d %q; want %q", response.Code, response.Body.String(), want)
	}
	assertHLSSessionTimelineParity(t, serve, path, duration, want)
	original, err := os.ReadFile(path)
	if err != nil || string(original) != manifest {
		t.Fatalf("serving mutated cached manifest: %q, %v", original, err)
	}
}

func assertHLSSessionTimelineParity(t *testing.T, serve func(http.ResponseWriter, *http.Request, string, int, float64) bool, path string, duration float64, want string) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/item/index.m3u8?playSessionId=playback-session", nil)
	response := httptest.NewRecorder()
	if !serve(response, request, path, 0, duration) {
		t.Fatal("valid session bypassed manifest handling")
	}
	plain := strings.ReplaceAll(response.Body.String(), "?playSessionId=playback-session", "")
	plain = strings.ReplaceAll(plain, "#EXT-X-START:TIME-OFFSET=0,PRECISE=YES\n", "")
	if response.Code != http.StatusOK || plain != want {
		t.Fatalf("session timeline = %d %q; want %q", response.Code, plain, want)
	}
}
