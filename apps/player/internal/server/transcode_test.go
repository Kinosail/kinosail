package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func transcodeFixture() servertest.TranscodeFixture {
	return servertest.TranscodeFixture{New: func(config servertest.TranscodeConfig) http.Handler {
		return server.New(server.Config{MediaDir: config.MediaDir, CacheDir: config.CacheDir, FFmpeg: config.FFmpeg, FFprobe: config.FFprobe})
	}, WriteExecutable: writeExecutable, AssertSafari: assertSafariDefersMatroska, PlayableHLS: fakePlayableHLS(), PlayerScript: "/static/player.js?v=73"}
}

func TestUnsupportedContainerDefaultsToDirectPlaybackWithCompatibleFallback(t *testing.T) {
	transcodeFixture().UnsupportedContainerDefaultsToDirectPlaybackWithCompatibleFallback(t)
}

func TestUnsupportedCodecInMP4DefaultsToDirectPlaybackWithCompatibleFallback(t *testing.T) {
	transcodeFixture().UnsupportedCodecInMP4DefaultsToDirectPlaybackWithCompatibleFallback(t)
}

func TestViewerCanRequestSeekableCompatiblePlaylist(t *testing.T) {
	transcodeFixture().ViewerCanRequestSeekableCompatiblePlaylist(t)
}

func TestInterruptedCompatibleCacheIsRegeneratedAfterServerRestart(t *testing.T) {
	transcodeFixture().InterruptedCompatibleCacheIsRegeneratedAfterServerRestart(t)
}

func TestAdaptiveHLSPublishesACompleteTruthfulLadder(t *testing.T) { //nolint:funlen // The public manifest and encoder contract are one playback outcome.
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"hevc\",\"width\":1920,\"height\":1080,\"r_frame_rate\":\"24/1\"},{\"codec_type\":\"audio\",\"codec_name\":\"aac\"}],\"format\":{\"format_name\":\"matroska\",\"bit_rate\":\"10000000\"}}'\n")
	arguments, ffmpeg := filepath.Join(tools, "arguments"), filepath.Join(tools, "ffmpeg")
	writeExecutable(t, ffmpeg, "#!/bin/sh\nprintf '%s\\n' \"$*\" > '"+arguments+"'\n"+fakePlayableHLS())
	handler, id := firstWebItem(t, server.Config{MediaDir: media, CacheDir: cache, FFprobe: ffprobe, FFmpeg: ffmpeg})
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id+"?compatible=1", nil))
	match := regexp.MustCompile(`data-hls="([^"]+)"`).FindStringSubmatch(page.Body.String())
	if len(match) != 2 {
		t.Fatalf("player source = %q", page.Body.String())
	}
	playlist := httptest.NewRecorder()
	handler.ServeHTTP(playlist, httptest.NewRequestWithContext(t.Context(), http.MethodGet, match[1], nil))
	manifest, used := playlist.Body.String(), readTestFile(t, arguments)
	for _, expected := range []string{"#EXT-X-INDEPENDENT-SEGMENTS", "BANDWIDTH=14,AVERAGE-BANDWIDTH=14", `CODECS="avc1.64002A,mp4a.40.2"`, "RESOLUTION=640x360,FRAME-RATE=24", "540p/index.m3u8", "720p/index.m3u8", "1080p/index.m3u8"} {
		if !strings.Contains(manifest, expected) {
			t.Fatalf("manifest lacks %q: %q", expected, manifest)
		}
	}
	for _, expected := range []string{"-force_key_frames expr:gte(t,n_forced*2)", "-hls_time 2", "-hls_flags temp_file+independent_segments", "-hls_segment_options movflags=+frag_discont+skip_sidx", "-profile:v high", "-level:v 4.2", "-filter_threads 1", "-filter_complex_threads 1"} {
		if !strings.Contains(used, expected) {
			t.Fatalf("FFmpeg arguments lack %q: %q", expected, used)
		}
	}
	if threads := strings.Count(used, "-threads:v "); threads != 3 {
		t.Fatalf("adaptive presentation has %d bounded encoders, want 3: %q", threads, used)
	}
}

func TestAdaptiveHLSDoesNotAdvertiseOrEncodeAbsentAudio(t *testing.T) {
	transcodeFixture().AdaptiveHLSDoesNotAdvertiseOrEncodeAbsentAudio(t)
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	return servertest.ReadTranscodeFile(t, path)
}
