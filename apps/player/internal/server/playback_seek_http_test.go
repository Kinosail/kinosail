package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

// The fake tools admit requests and record encoder effects; the hosted coded
// fixture owns actual frame300..767 delivery and browser presentation proof.
func seekPlanHTTPFixture(t *testing.T) (http.Handler, string, string) {
	t.Helper()
	media, data, cache, tools := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(tools, "probe")
	writeExecutable(t, probe, `#!/bin/sh
printf '%s' '{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":640,"height":360,"r_frame_rate":"24/1"},{"index":1,"codec_type":"audio","codec_name":"aac","sample_rate":"48000","channels":2,"disposition":{"default":1}}],"format":{"format_name":"matroska","duration":"32.021"}}'
`)
	encoder, calls := filepath.Join(tools, "encoder"), filepath.Join(tools, "encoder-called")
	writeExecutable(t, encoder, "#!/bin/sh\nprintf 'called' > '"+calls+"'\nexit 1\n")
	handler, id := firstWebItem(t, server.Config{MediaDir: media, DataDir: data, CacheDir: cache, FFprobe: probe, FFmpeg: encoder})
	return handler, id, calls
}

func TestPlaybackSeekPlanReportsActualConversionBeforeMedia(t *testing.T) {
	handler, id, calls := seekPlanHTTPFixture(t)
	for _, test := range []struct{ query, mode, prefix string }{
		{"", "remux", "r-"}, {"?position=0", "remux", "r-"}, {"?position=12.5", "transcode", "t-"},
	} {
		response := serveRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/"+id+"/playback"+test.query, nil))
		var result struct {
			Compatible      string
			CompatibleLabel string
			CompatiblePlan  struct{ Mode string }
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || result.CompatiblePlan.Mode != test.mode || !strings.Contains(result.Compatible, "/p/"+test.prefix) {
			t.Fatalf("seek %s = %d, plan=%s source=%s", test.query, response.Code, result.CompatiblePlan.Mode, result.Compatible)
		}
		if test.mode == "transcode" && (result.CompatibleLabel != "Transcoding video" || strings.Contains(result.Compatible, "-o")) {
			t.Fatal("seek conversion label or full-origin native source disagrees with plan")
		}
	}
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Fatal("planning started an encoder")
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/progress/"+id, strings.NewReader("seconds=12.5&duration=32.021"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if response := serveRequest(handler, request); response.Code != http.StatusNoContent {
		t.Fatalf("save resume = %d", response.Code)
	}
	for _, query := range []string{"?compatible=1", "?direct=1"} {
		response := serveRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id+query, nil))
		body := response.Body.String()
		if query == "?compatible=1" && (!strings.Contains(body, `data-compatibility-mode="transcode"`) || !strings.Contains(body, `/p/t-`) || !strings.Contains(body, "Transcoding video")) {
			t.Fatal("saved web resume does not report its conversion/source")
		}
		if query == "?direct=1" && (!strings.Contains(body, `data-playback-label="Direct Play"`) || !strings.Contains(body, `/media/`+id)) {
			t.Fatal("explicit Direct was changed by exact compatible seek policy")
		}
	}
}

func TestPlaybackSeekRejectsAmbiguousAndInvalidInputsBeforeWork(t *testing.T) {
	handler, id, calls := seekPlanHTTPFixture(t)
	for _, query := range []string{"position=", "position=-0", "position=-1", "position=NaN", "position=Infinity", "position=12.55",
		"position=32.021", "position=604801", "position=12.5&position=12.5", "position=%FF", "position=" + strings.Repeat("1", 4097),
		"position=12.5&recipe=unknown", "recipe=r-a0-s0-none-t0-b0", "position=12.5&recipe=r-a0-s0-none-t0-b0-o12000",
		"position=12.5&recipe=r-a1-s0-none-t0-b0", "position=12.5&recipe=r-a0-s0-none-t0-b0&recipe=r-a0-s0-none-t0-b0",
		"position=12.5&recipe=a-a0-s0-none-t0-b0-e1", "position=12.5&recipe=" + strings.Repeat("a", 2049)} {
		response := serveRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/"+id+"/playback?"+query, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid seek query admitted with status %d", response.Code)
		}
		if _, err := os.Stat(calls); !os.IsNotExist(err) {
			t.Fatal("rejected seek started an encoder")
		}
	}
	for _, child := range []string{"index.m3u8", "360p/index.m3u8", "360p/init.mp4", "360p/segment-00000.m4s"} {
		response := serveRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet,
			"/hls/"+id+"/p/r-a0-s0-none-t0-b0-o12500/"+child, nil))
		if response.Code != http.StatusConflict {
			t.Fatalf("uncertified copied seek child %s = %d", child, response.Code)
		}
		if _, err := os.Stat(calls); !os.IsNotExist(err) {
			t.Fatal("old copied recipe started conversion under a copy token")
		}
	}
}
