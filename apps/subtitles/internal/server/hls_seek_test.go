package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestCompatiblePlaybackGeneratesAnAlignedSeekableSuffix(t *testing.T) {
	t.Parallel()
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"video","codec_name":"h264","profile":"High","level":40,"width":1920,"height":1080},{"codec_type":"audio","codec_name":"truehd"}],"format":{"format_name":"mp4","duration":"7200"}}'
`)
	arguments, ffmpeg := filepath.Join(tools, "arguments"), filepath.Join(tools, "ffmpeg")
	writeExecutable(t, ffmpeg, fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> '%s'\n", arguments)+fakePlayableHLS())
	handler, id := firstWebItem(t, server.Config{MediaDir: media, CacheDir: cache, FFprobe: ffprobe, FFmpeg: ffmpeg})
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id+"?compatible=1", nil))
	source := regexp.MustCompile(`data-hls="([^"]+)"`).FindStringSubmatch(page.Body.String())[1]
	seekSource := strings.Replace(source, "/index.m3u8", "-o2940000/index.m3u8", 1)
	playlist := httptest.NewRecorder()
	handler.ServeHTTP(playlist, httptest.NewRequestWithContext(t.Context(), http.MethodGet, seekSource, nil))
	used, err := os.ReadFile(arguments)
	if playlist.Code != http.StatusOK || err != nil || !strings.Contains(string(used), "-ss 2940") {
		t.Fatalf("seek playlist = %d, arguments = %q, error = %v", playlist.Code, used, err)
	}
}

func TestCompatiblePlaybackRejectsInvalidSeekOffsetsWithoutEncoding(t *testing.T) {
	t.Parallel()
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\"}],\"format\":{\"format_name\":\"mp4\",\"duration\":\"120\"}}'\n")
	called, ffmpeg := filepath.Join(tools, "called"), filepath.Join(tools, "ffmpeg")
	writeExecutable(t, ffmpeg, "#!/bin/sh\ntouch '"+called+"'\n"+fakePlayableHLS())
	handler, id := firstWebItem(t, server.Config{MediaDir: media, CacheDir: cache, FFprobe: ffprobe, FFmpeg: ffmpeg})
	for _, token := range []string{"o", "oabc", "x30000", "o1", "o30001", "o30000-o60000", "o120000", "o604800001"} {
		response := httptest.NewRecorder()
		path := "/hls/" + id + "/p/t-a0-s0-none-t0-b0-" + token + "/index.m3u8"
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound && response.Code != http.StatusBadRequest {
			t.Fatalf("invalid offset %q = %d %q", token, response.Code, response.Body.String())
		}
	}
	if _, err := os.Stat(called); !os.IsNotExist(err) {
		t.Fatalf("invalid seek started FFmpeg: %v", err)
	}
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid seek changed cache: %v, %v", entries, err)
	}
}
