package server_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/playback"
)

// A clip longer than the startup burst must remain deliverable at the fastest
// supported speed. Sequential requests prevent a distant seek from hiding a
// slow producer behind another startup burst.
func TestRealHLSGenerationKeepsAheadOfSupportedPlaybackSpeeds(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for the HLS speed integration test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required for the HLS speed integration test")
	}
	for _, sample := range []struct {
		name, video, audio, mode string
		skip, resume             bool
	}{
		{"remux", "libx264", "aac", "remux", false, false},
		{"audio conversion", "libx264", "ac3", "audio-transcode", false, false},
		{"video conversion", "mpeg4", "aac", "transcode", false, false},
		{"automatic skip", "libx264", "aac", "audio-transcode", true, false},
		{"resume", "libx264", "aac", "remux", false, true},
	} {
		t.Run(sample.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			media := speedTestMedia(t, ffmpeg, sample.video, sample.audio, sample.skip)
			cache := t.TempDir()
			logs := captureCopiedLogs(t, 64<<10)
			handler, id := formatTestItem(t, server.Config{Lifecycle: ctx, MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe})
			handler = &speedDiagnosticHandler{Handler: handler, cache: cache, logs: logs}
			if sample.skip {
				assertAPIBody(t, apiCall(t, handler, "", http.MethodPut, "/api/v1/settings/playback", map[string]any{"mode": "compatible", "autoplay": true, "subtitles": "on", "autoSkip": []string{"intro"}}), http.StatusOK)
			}
			var info struct {
				Compatible     string
				CompatiblePlan playback.PlaybackPlan
			}
			mustJSON(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback?videoCodecs=h264&audioCodecs=aac", nil), &info)
			if info.CompatiblePlan.Mode != sample.mode {
				t.Fatalf("fixture plan = %+v; want %s", info.CompatiblePlan, sample.mode)
			}
			// Consume a long, complete window before the final muxer tail.
			duration := 60.0
			if sample.skip {
				duration -= 10
			}
			if sample.resume {
				info.Compatible = strings.Replace(info.Compatible, "/index.m3u8", "-o12000/index.m3u8", 1)
				duration -= 12
			}
			assertSpeedTestDelivery(t, ctx, handler, info.Compatible, ffmpeg, duration)
		})
	}
}

func speedTestMedia(t *testing.T, ffmpeg, video, audio string, skip bool) string {
	t.Helper()
	media := t.TempDir()
	arguments := []string{"-v", "error", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24:duration=72", "-f", "lavfi", "-i", "sine=frequency=1000:sample_rate=48000:duration=72"}
	if skip {
		metadata := filepath.Join(media, "chapters.ffmeta")
		if err := os.WriteFile(metadata, []byte(";FFMETADATA1\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=10000\nEND=20000\ntitle=Intro\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		arguments = append(arguments, "-f", "ffmetadata", "-i", metadata, "-map_metadata", "2")
	}
	arguments = append(arguments, "-map", "0:v:0", "-map", "1:a:0", "-c:v", video, "-g", "48", "-c:a", audio)
	if video == "libx264" {
		arguments = append(arguments, "-preset", "ultrafast", "-sc_threshold", "0")
	}
	arguments = append(arguments, filepath.Join(media, "Episode.S01E01.mkv"))
	command := exec.CommandContext(t.Context(), ffmpeg, arguments...) //nolint:gosec // Tool comes from LookPath and all arguments are fixed synthetic-test inputs.
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate speed fixture: %v: %s", err, output)
	}
	return media
}

func speedTestGET(t *testing.T, ctx context.Context, handler http.Handler, path string) []byte {
	t.Helper()
	var before []speedCacheFact
	diagnostic, observed := handler.(*speedDiagnosticHandler)
	parts := strings.Split(path, "/")
	target := strings.Join(parts[max(0, len(parts)-2):], "/")
	if observed {
		before = speedFailureCacheFacts(diagnostic.cache, target)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if observed {
		diagnostic.requests++
		request.Header.Set("X-Request-ID", fmt.Sprintf("hls-speed-%d", diagnostic.requests))
		request.Header.Set("X-Playback-Session", "hls-speed-fixture")
	}
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.Len() == 0 {
		if observed {
			beforeJSON, _ := json.Marshal(before)
			afterJSON, _ := json.Marshal(speedFailureCacheFacts(diagnostic.cache, target))
			t.Logf("HLS speed failed-request cache observations (not admission decisions): before=%s after=%s", beforeJSON, afterJSON)
			speedFailureOperations(t, diagnostic.logs, request.Header.Get("X-Request-ID"))
			t.Fatalf("HLS delivery %s = %d (canceled=%t; body_bytes=%d; body_SHA256=%x)", target, response.Code, ctx.Err() != nil, response.Body.Len(), sha256.Sum256(response.Body.Bytes()))
		}
		t.Fatalf("HLS delivery %s = %d (%s): %s", path, response.Code, ctx.Err(), response.Body.String())
	}
	return response.Body.Bytes()
}

func speedTestURIs(manifest []byte) []string {
	var result []string
	for _, line := range strings.Split(string(manifest), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			result = append(result, line)
		}
	}
	return result
}

func assertSpeedTestDelivery(t *testing.T, ctx context.Context, handler http.Handler, compatible, ffmpeg string, duration float64) {
	t.Helper()
	started := time.Now()
	delivery, stop := context.WithTimeout(ctx, time.Duration(duration/3)*time.Second)
	defer stop()
	master := speedTestGET(t, delivery, handler, compatible)
	base := compatible[:strings.LastIndex(compatible, "/")+1]
	variants := speedTestURIs(master)
	if len(variants) == 0 {
		t.Fatal("HLS master has no renditions")
	}
	for _, variant := range variants {
		playlist := speedTestGET(t, delivery, handler, base+variant)
		rendition := base + variant[:strings.LastIndex(variant, "/")+1]
		fragments := speedTestGET(t, delivery, handler, rendition+"init.mp4")
		segments := speedTestURIs(playlist)
		if len(segments) < int(duration/2) {
			t.Fatalf("rendition has only %d segments for %.0f seconds", len(segments), duration)
		}
		for _, segment := range segments[:int(duration/2)] {
			fragments = append(fragments, speedTestGET(t, delivery, handler, rendition+segment)...)
		}
		decode := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-i", "pipe:0", "-map", "0:v:0", "-map", "0:a:0", "-f", "null", "-") //nolint:gosec // Tool comes from LookPath and all arguments are fixed synthetic-test inputs.
		decode.Stdin = bytes.NewReader(fragments)
		if output, err := decode.CombinedOutput(); err != nil {
			t.Fatalf("delivered rendition does not decode: %v: %s", err, output)
		}
	}
	elapsed := time.Since(started)
	for _, rate := range []float64{0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.5, 3} {
		if elapsed.Seconds() > duration/rate {
			t.Errorf("%.2fx: delivered %.0f seconds in %s; playback would exhaust its buffer", rate, duration, elapsed)
		}
	}
	t.Logf("%.0f media seconds, %d renditions delivered and decoded in %s", duration, len(variants), elapsed)
}
