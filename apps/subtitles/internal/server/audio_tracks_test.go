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
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestViewerCanSelectEmbeddedAudioTrack(t *testing.T) {
	t.Parallel()

	mediaDir, toolsDir, cacheDir := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(toolsDir, "ffprobe")
	probeScript := `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1080},{"codec_type":"audio","codec_name":"aac","tags":{"language":"eng"}},{"codec_type":"audio","codec_name":"ac3","tags":{"language":"spa"}}],"format":{"duration":"120"}}'
`
	writeExecutable(t, ffprobe, probeScript)
	arguments := filepath.Join(toolsDir, "arguments")
	ffmpeg := filepath.Join(toolsDir, "ffmpeg")
	encodeScript := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> '%s'\n", arguments) + fakePlayableHLS()
	writeExecutable(t, ffmpeg, encodeScript)
	handler := server.New(server.Config{MediaDir: mediaDir, CacheDir: cacheDir, FFmpeg: ffmpeg, FFprobe: ffprobe})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id+"?compatible=1&audio=1", nil))
	playlist := httptest.NewRecorder()
	handler.ServeHTTP(playlist, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+id+"/audio/1/index.m3u8", nil))
	used, err := os.ReadFile(arguments)

	if err != nil || playlist.Code != http.StatusOK || !strings.Contains(player.Body.String(), "SPA · AC3") || !strings.Contains(player.Body.String(), "/hls/"+id+"/p/a-a1-s0-none-t0-b0/index.m3u8") || !strings.Contains(string(used), "-map 0:a:1?") || !strings.Contains(string(used), "-c:v copy") {
		t.Fatalf("player = %q, playlist = %d, arguments = %q, error = %v", player.Body.String(), playlist.Code, used, err)
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	servertest.WriteExecutable(t, path, content)
}

func fakePlayableHLS() string { return servertest.SingleSegmentPlayableHLS() }
