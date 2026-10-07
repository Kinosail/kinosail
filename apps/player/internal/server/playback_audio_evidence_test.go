package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestAutomaticSkipPreservesSelectedAudioCompatibilityEvidence(t *testing.T) { //nolint:cyclop,gocognit // One API and web contract checks audio selection against the same skip timeline.
	t.Parallel()
	for _, sample := range []struct {
		codec    string
		required bool
	}{{"dts", true}, {"aac", false}, {"", false}} {
		t.Run(sample.codec, func(t *testing.T) {
			media, data, tools := t.TempDir(), t.TempDir(), t.TempDir()
			if err := os.WriteFile(filepath.Join(media, "Episode.mkv"), []byte("media"), 0o600); err != nil {
				t.Fatal(err)
			}
			ffprobe := filepath.Join(tools, "ffprobe")
			probe := fmt.Sprintf(`{"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1080},{"codec_type":"audio","codec_name":%q}],"format":{"format_name":"matroska","duration":"120"}}`, sample.codec)
			writeExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '"+probe+"'\n")
			handler, id := firstWebItem(t, server.Config{Lifecycle: t.Context(), MediaDir: media, DataDir: data, FFprobe: ffprobe})
			marker := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/markers/"+id, strings.NewReader("type=intro&start=10&end=20"))
			marker.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if response := serveRequest(handler, marker); response.Code != http.StatusSeeOther {
				t.Fatalf("add marker: %d %s", response.Code, response.Body.String())
			}
			assertAPIBody(t, apiCall(t, handler, "", http.MethodPut, "/api/v1/items/"+id+"/progress", map[string]any{"seconds": 42}), http.StatusOK)
			var info struct {
				Duration, Start      float64
				ProgressToken        string
				AutoSkip             []string
				Plan, CompatiblePlan struct {
					Mode                       string
					AudioCompatibilityRequired bool
				}
			}
			mustJSON(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback", nil), &info)
			mode, duration, start := "direct", 120.0, 42.0
			if sample.required {
				mode, duration, start = "transcode", 110, 32
			}
			if info.Plan.Mode != mode || info.CompatiblePlan.Mode != "transcode" || info.Plan.AudioCompatibilityRequired != sample.required || info.CompatiblePlan.AudioCompatibilityRequired != sample.required {
				t.Fatalf("selected audio evidence lost behind skip plan: %+v", info)
			}
			if info.Duration != duration || info.Start != start || sample.required && (info.ProgressToken == "" || len(info.AutoSkip) != 0) {
				t.Fatalf("audio recovery did not project the skip timeline: %+v", info)
			}
			page := serveRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
			if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), fmt.Sprintf(`data-audio-compatibility-required="%t"`, sample.required)) {
				t.Fatalf("web player omitted audio evidence: %d", page.Code)
			}
			if sample.required && (!strings.Contains(page.Body.String(), `data-hls="/hls/`) || !strings.Contains(page.Body.String(), `data-start="32"`) || strings.Contains(page.Body.String(), `data-adaptive=`)) {
				t.Fatal("audio recovery did not start with the planned stream and resume time")
			}
		})
	}
}
