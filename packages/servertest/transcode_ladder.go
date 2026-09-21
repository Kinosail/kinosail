package servertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/workload"
)

func (fixture TranscodeFixture) AdaptiveHLSPublishesACompleteTruthfulLadder(t *testing.T, segmentSeconds string, appArguments ...string) { //nolint:funlen // The public manifest and encoder contract are one playback outcome.
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	WriteExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"hevc\",\"width\":1920,\"height\":1080,\"r_frame_rate\":\"24/1\"},{\"codec_type\":\"audio\",\"codec_name\":\"aac\"}],\"format\":{\"format_name\":\"matroska\",\"bit_rate\":\"10000000\"}}'\n")
	arguments, ffmpeg := filepath.Join(tools, "arguments"), filepath.Join(tools, "ffmpeg")
	WriteExecutable(t, ffmpeg, "#!/bin/sh\nprintf '%s\\n' \"$*\" > '"+arguments+"'\n"+PlayableHLS())
	handler, id := FirstWebItem(t, fixture.New(TranscodeConfig{MediaDir: media, CacheDir: cache, FFprobe: ffprobe, FFmpeg: ffmpeg}))
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id+"?compatible=1", nil))
	match := regexp.MustCompile(`data-hls="([^"]+)"`).FindStringSubmatch(page.Body.String())
	if len(match) != 2 {
		t.Fatalf("player source = %q", page.Body.String())
	}
	playlist := httptest.NewRecorder()
	handler.ServeHTTP(playlist, httptest.NewRequestWithContext(t.Context(), http.MethodGet, match[1], nil))
	manifest, used := playlist.Body.String(), ReadTranscodeFile(t, arguments)
	for _, expected := range []string{"#EXT-X-INDEPENDENT-SEGMENTS", "BANDWIDTH=14,AVERAGE-BANDWIDTH=14", `CODECS="avc1.64002A,mp4a.40.2"`, "RESOLUTION=640x360,FRAME-RATE=24"} {
		if !strings.Contains(manifest, expected) {
			t.Fatalf("manifest lacks %q: %q", expected, manifest)
		}
	}
	for _, expected := range append(appArguments, []string{"-force_key_frames expr:gte(t,n_forced*2)", "-hls_time " + segmentSeconds, "-hls_flags temp_file+independent_segments", "-profile:v high", "-level:v 4.2", "-filter_threads 1", "-filter_complex_threads 1"}...) {
		if !strings.Contains(used, expected) {
			t.Fatalf("FFmpeg arguments lack %q: %q", expected, used)
		}
	}
	assertAdaptiveRenditions(t, manifest, used)
}

func assertAdaptiveRenditions(t *testing.T, manifest, used string) {
	t.Helper()
	capacity := workload.HeavyCapacity()
	qualities := []string{"360p", "432p", "540p", "720p", "1080p"}
	for _, quality := range qualities[len(qualities)-capacity:] {
		if !strings.Contains(manifest, quality+"/index.m3u8") {
			t.Fatalf("manifest lacks %s: %s", quality, manifest)
		}
	}
	if strings.Count(manifest, "#EXT-X-STREAM-INF:") != capacity || strings.Count(used, "-threads:v ") != capacity {
		t.Fatalf("advertised and encoded renditions must match capacity %d: manifest=%s arguments=%s", capacity, manifest, used)
	}
}
