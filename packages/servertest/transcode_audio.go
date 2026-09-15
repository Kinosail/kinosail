package servertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func (fixture TranscodeFixture) AdaptiveHLSDoesNotAdvertiseOrEncodeAbsentAudio(t *testing.T) {
	t.Helper()
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Silent.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	fixture.WriteExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"hevc\",\"width\":640,\"height\":360}]}'\n")
	arguments, ffmpeg := filepath.Join(tools, "arguments"), filepath.Join(tools, "ffmpeg")
	fixture.WriteExecutable(t, ffmpeg, "#!/bin/sh\nprintf '%s\\n' \"$*\" > '"+arguments+"'\n"+PlayableHLSWithMedia(640, 360, ""))
	handler, id := FirstWebItem(t, fixture.New(TranscodeConfig{MediaDir: media, CacheDir: cache, FFprobe: ffprobe, FFmpeg: ffmpeg}))
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id+"?compatible=1", nil))
	source := regexp.MustCompile(`data-hls="([^"]+)"`).FindStringSubmatch(page.Body.String())[1]
	playlist := httptest.NewRecorder()
	handler.ServeHTTP(playlist, httptest.NewRequestWithContext(t.Context(), http.MethodGet, source, nil))
	used := ReadTranscodeFile(t, arguments)
	if playlist.Code != http.StatusOK || !strings.Contains(playlist.Body.String(), `CODECS="avc1.64002A"`) || strings.Contains(playlist.Body.String(), "mp4a") || strings.Contains(used, "-c:a") {
		t.Fatalf("silent manifest = %d %q, FFmpeg = %q", playlist.Code, playlist.Body.String(), used)
	}
}

func ReadTranscodeFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
