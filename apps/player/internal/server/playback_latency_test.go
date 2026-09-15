package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestPlaybackAdaptersBoundColdRandomAccessProbe(t *testing.T) { //nolint:gocognit // Both adapters must prove the same cold-probe boundary.
	for _, adapter := range []string{"api", "web"} {
		t.Run(adapter, func(t *testing.T) {
			media, data, tools := t.TempDir(), t.TempDir(), t.TempDir()
			if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("video"), 0o600); err != nil {
				t.Fatal(err)
			}
			arguments, ffprobe := filepath.Join(tools, "arguments"), filepath.Join(tools, "ffprobe")
			writeExecutable(t, ffprobe, fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> %q
case " $* " in
  *" -show_frames "*)
    case " $* " in *" -read_intervals "*) ;; *) sleep 2 ;; esac
    printf '%%s' '{"frames":[{"key_frame":1,"best_effort_timestamp_time":"0"},{"key_frame":1,"best_effort_timestamp_time":"10"},{"key_frame":1,"best_effort_timestamp_time":"20"},{"key_frame":1,"best_effort_timestamp_time":"60"}]}' ;;
  *) printf '%%s' '{"streams":[{"codec_type":"video","codec_name":"h264","profile":"High","level":40,"width":1920,"height":1080,"r_frame_rate":"24/1"},{"codec_type":"audio","codec_name":"aac","profile":"LC"}],"format":{"format_name":"mp4","duration":"60"}}' ;;
esac
`, arguments))
			handler, id := firstWebItem(t, server.Config{MediaDir: media, DataDir: data, FFprobe: ffprobe})
			marker := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/markers/"+id, strings.NewReader("type=intro&start=10&end=20"))
			marker.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if response := serveRequest(handler, marker); response.Code != http.StatusSeeOther {
				t.Fatalf("add marker = %d %q", response.Code, response.Body.String())
			}

			started := time.Now()
			response := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil)
			if adapter == "api" {
				request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/"+id+"/playback", nil)
			}
			handler.ServeHTTP(response, request)
			if elapsed := time.Since(started); response.Code != http.StatusOK || elapsed > time.Second {
				t.Fatalf("cold %s playback = %d in %v: %q", adapter, response.Code, elapsed, response.Body.String())
			}
			used, err := os.ReadFile(arguments)
			if err != nil || !strings.Contains(string(used), "-show_frames") || !strings.Contains(string(used), "-read_intervals") {
				t.Fatalf("FFprobe arguments = %q, error = %v", used, err)
			}
		})
	}
}
