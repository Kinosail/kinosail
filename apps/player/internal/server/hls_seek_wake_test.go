package server_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
)

func TestCompatiblePlaybackSeekRespondsWhenTheRequestedSegmentIsPublished(t *testing.T) {
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe, ffmpeg, stamps := filepath.Join(tools, "ffprobe"), filepath.Join(tools, "ffmpeg"), filepath.Join(tools, "stamps")
	writeExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"hevc\",\"width\":1920,\"height\":1080},{\"codec_type\":\"audio\",\"codec_name\":\"aac\"}],\"format\":{\"format_name\":\"matroska\",\"duration\":\"7200\"}}'\n")
	writeExecutable(t, ffmpeg, fmt.Sprintf(`#!/bin/sh
start=0
previous=
for argument do
  if [ "$previous" = "-start_number" ]; then start=$argument; fi
  previous=$argument
done
for output do
  case "$output" in
    */segment-%%05d.m4s)
      directory=${output%%/*}; mkdir -p "$directory"
      if [ "$start" != 0 ]; then sleep 0.0$((start %% 7 + 2)); fi
      segment=$(printf 'segment-%%05d.m4s' "$start")
      printf segment > "$directory/$segment.tmp"
      if [ "$start" != 0 ]; then date +%%s%%N > '%[1]s/'"$start.tmp"; fi
      mv "$directory/$segment.tmp" "$directory/$segment"
      if [ "$start" != 0 ]; then date +%%s%%N >> '%[1]s/'"$start.tmp"; mv '%[1]s/'"$start.tmp" '%[1]s/'"$start"; fi
      if [ "$start" = 0 ]; then printf segment > "$directory/segment-00001.m4s"; fi
      ;;
    */index.m3u8)
      directory=${output%%/*}; mkdir -p "$directory"
      %[2]s > "$directory/init.mp4"
      segment=$(printf 'segment-%%05d.m4s' "$start")
      printf '#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-MAP:URI="init.mp4"\n#EXTINF:4,\n%%s\n' "$segment" > "$output"
      ;;
  esac
done
while :; do sleep .02; done
`, stamps, mp4fixture.Shell(mp4fixture.Initialization(1920, 1080, "h264", "aac", ""))))
	if err := os.Mkdir(stamps, 0o700); err != nil {
		t.Fatal(err)
	}
	lifecycle, stop := context.WithCancel(t.Context())
	t.Cleanup(func() { stop(); settleDirectory(t, cache) })
	handler, id := firstWebItem(t, server.Config{Lifecycle: lifecycle, MediaDir: media, CacheDir: cache, FFprobe: ffprobe, FFmpeg: ffmpeg})
	base := "/hls/" + id + "/p/t-a0-s0-none-t0-b0/"
	master := httptest.NewRecorder()
	handler.ServeHTTP(master, httptest.NewRequestWithContext(t.Context(), http.MethodGet, base+"index.m3u8", nil))
	if master.Code != http.StatusOK {
		t.Fatalf("master = %d %q", master.Code, master.Body.String())
	}
	var latencies []time.Duration
	for number := 60; number < 60+12*15; number += 15 {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(ctx, http.MethodGet, base+fmt.Sprintf("1080p/segment-%05d.m4s", number), nil))
		answered := time.Now()
		cancel()
		stamp, err := os.ReadFile(filepath.Join(stamps, strconv.Itoa(number)))
		for deadline := time.Now().Add(2 * time.Second); err != nil && time.Now().Before(deadline); stamp, err = os.ReadFile(filepath.Join(stamps, strconv.Itoa(number))) {
			time.Sleep(time.Millisecond)
		}
		published, valid := publicationMidpoint(stamp)
		if response.Code != http.StatusOK || response.Body.String() != "segment" || err != nil || !valid {
			t.Fatalf("segment %d = %d %q, stamp = %q, %v", number, response.Code, response.Body.String(), stamp, err)
		}
		latencies = append(latencies, answered.Sub(published))
	}
	slices.Sort(latencies)
	t.Logf("seek response after segment publication: median = %v, latencies = %v", latencies[len(latencies)/2], latencies)
	if median := latencies[len(latencies)/2]; median > 5*time.Millisecond {
		t.Fatalf("median seek response after publication = %v, latencies = %v", median, latencies)
	}
}

func settleDirectory(t *testing.T, directory string) {
	t.Helper()
	quiet := 0
	for deadline := time.Now().Add(5 * time.Second); quiet < 5 && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if _, err := os.Stat(directory); os.IsNotExist(err) {
			quiet++
			continue
		}
		quiet = 0
		_ = os.RemoveAll(directory)
	}
}

func publicationMidpoint(stamp []byte) (time.Time, bool) {
	fields := strings.Fields(string(stamp))
	if len(fields) != 2 {
		return time.Time{}, false
	}
	before, beforeErr := strconv.ParseInt(fields[0], 10, 64)
	after, afterErr := strconv.ParseInt(fields[1], 10, 64)
	return time.Unix(0, before+(after-before)/2), beforeErr == nil && afterErr == nil && after >= before
}
