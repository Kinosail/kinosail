package server_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
)

func TestCompatiblePlaybackRestartsAtADistantRequestedSegment(t *testing.T) { //nolint:funlen // The process fixture proves the complete segment-request restart path.
	t.Parallel()
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"hevc\",\"width\":1920,\"height\":1080},{\"codec_type\":\"audio\",\"codec_name\":\"aac\"}],\"format\":{\"format_name\":\"matroska\",\"duration\":\"7200\"}}'\n")
	arguments, ffmpeg := filepath.Join(tools, "arguments"), filepath.Join(tools, "ffmpeg")
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> '%s'
start=0
format=
previous=
for argument do
  if [ "$previous" = "-start_number" ]; then start=$argument; fi
  if [ "$previous" = "-f" ]; then format=$argument; fi
  previous=$argument
done
for output do
  case "$output" in
    */segment-%%05d.m4s)
      directory=${output%%/*}; mkdir -p "$directory"
      segment=$(printf 'segment-%%05d.m4s' "$start")
      printf segment > "$directory/$segment"
      if [ "$start" = 0 ]; then printf segment > "$directory/segment-00001.m4s"; fi
      ;;
    */index.m3u8)
      directory=${output%%/*}; mkdir -p "$directory"
      %s > "$directory/init.mp4"
      segment=$(printf 'segment-%%05d.m4s' "$start")
      printf '#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-MAP:URI="init.mp4"\n#EXTINF:4,\n%%s\n' "$segment" > "$output"
      if [ "$start" = 0 ]; then
        printf '#EXTINF:4,\nsegment-00001.m4s\n' >> "$output"
	  else
		printf '#EXT-X-ENDLIST\n' >> "$output"
      fi
      ;;
  esac
done
if [ "$format" = hls ] && [ "$start" = 0 ]; then while :; do sleep .02; done; fi
`, arguments, mp4fixture.Shell(mp4fixture.Initialization(1920, 1080, "h264", "aac", "")))
	writeExecutable(t, ffmpeg, script)
	handler, id := firstWebItem(t, server.Config{Lifecycle: t.Context(), MediaDir: media, CacheDir: cache, FFprobe: ffprobe, FFmpeg: ffmpeg})
	master := httptest.NewRecorder()
	handler.ServeHTTP(master, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+id+"/p/t-a0-s0-none-t0-b0/index.m3u8", nil))
	requestContext, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	segment := httptest.NewRecorder()
	handler.ServeHTTP(segment, httptest.NewRequestWithContext(requestContext, http.MethodGet, "/hls/"+id+"/p/t-a0-s0-none-t0-b0/1080p/segment-00075.m4s", nil))
	used, err := os.ReadFile(arguments)
	if err != nil || master.Code != http.StatusOK || segment.Code != http.StatusOK {
		t.Fatalf("master = %d, segment = %d %q, arguments = %q, error = %v", master.Code, segment.Code, segment.Body.String(), used, err)
	}
	for _, expected := range []string{"-ss 300", "-output_ts_offset 300", "-avoid_negative_ts disabled", "-start_number 75"} {
		if !strings.Contains(string(used), expected) {
			t.Errorf("seek FFmpeg arguments lack %q: %q", expected, used)
		}
	}
	publicPlaylist, err := os.ReadFile(filepath.Join(cache, id, "1080p", "index.m3u8"))
	if err != nil || !strings.Contains(string(publicPlaylist), "segment-00000.m4s") || strings.Contains(string(publicPlaylist), "segment-00075.m4s") {
		t.Fatalf("public playlist changed during seek: %q, %v", publicPlaylist, err)
	}
	assertSeekRestartReusesCache(t, server.Config{Lifecycle: t.Context(), MediaDir: media, CacheDir: cache, FFprobe: ffprobe, FFmpeg: ffmpeg}, id, arguments, used)
}

func TestCompatiblePlaybackGeneratesAnAlignedSeekableSuffix(t *testing.T) {
	transcodeFixture().CompatiblePlaybackGeneratesAnAlignedSeekableSuffix(t, "2941100", "2941.1")
}

func TestCompatiblePlaybackRejectsInvalidSeekOffsetsWithoutEncoding(t *testing.T) {
	transcodeFixture().CompatiblePlaybackRejectsInvalidSeekOffsetsWithoutEncoding(t)
}

func TestCompatiblePlaybackRejectsInvalidResumeHintsWithoutEncoding(t *testing.T) {
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
	before := servertest.SnapshotHLSCache(t, cache)
	for _, query := range []string{"start=", "start=no", "start=-1", "start=0", "start=120", "start=604801", "start=9999999999", "start=30&start=60"} {
		response := httptest.NewRecorder()
		path := "/hls/" + id + "/p/t-a0-s0-none-t0-b0/index.m3u8?" + query
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid resume %q = %d %q", query, response.Code, response.Body.String())
		}
	}
	if _, err := os.Stat(called); !os.IsNotExist(err) {
		t.Fatalf("invalid resume started FFmpeg: %v", err)
	}
	if !reflect.DeepEqual(before, servertest.SnapshotHLSCache(t, cache)) {
		t.Fatal("invalid resume changed cache")
	}
}

func assertSeekRestartReusesCache(t *testing.T, config server.Config, id, arguments string, used []byte) {
	t.Helper()
	restarted := server.New(config)
	reloaded := httptest.NewRecorder()
	restarted.ServeHTTP(reloaded, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+id+"/p/t-a0-s0-none-t0-b0/index.m3u8", nil))
	afterRestart, err := os.ReadFile(arguments)
	if err != nil || reloaded.Code != http.StatusOK || strings.Count(string(afterRestart), " -f hls ") != strings.Count(string(used), " -f hls ") {
		t.Fatalf("restart master = %d, HLS calls before = %d, after = %d, error = %v", reloaded.Code, strings.Count(string(used), " -f hls "), strings.Count(string(afterRestart), " -f hls "), err)
	}
}
