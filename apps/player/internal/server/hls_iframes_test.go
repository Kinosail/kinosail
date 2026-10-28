package server_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestRealIFramePlaylistGivesAppleScrubbingThumbnails(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for the I-frame integration test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required for the I-frame integration test")
	}
	ctx := t.Context()
	media, cache := t.TempDir(), t.TempDir()
	source := filepath.Join(media, "Scrub Test (2020).mkv")
	if output, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=160x90:rate=24:duration=600", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=600",
		"-c:v", "libx264", "-g", "96", "-c:a", "aac", "-ac", "2", source).CombinedOutput(); err != nil {
		t.Fatalf("generate fixture: %v: %s", err, output)
	}
	handler, id := formatTestItem(t, server.Config{Lifecycle: ctx, MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe})
	var info struct{ Compatible string }
	mustJSON(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback?videoCodecs=h264&audioCodecs=aac", nil), &info)
	master := string(speedTestGET(t, ctx, handler, info.Compatible))
	stream := regexp.MustCompile(`(?m)^#EXT-X-I-FRAME-STREAM-INF:BANDWIDTH=\d+,CODECS="avc1\.4d401e",RESOLUTION=320x180,URI="iframes/index\.m3u8"$`)
	if !stream.MatchString(master) || !strings.Contains(master, "#EXT-X-STREAM-INF:") {
		t.Fatalf("master playlist has no I-frame stream:\n%s", master)
	}
	base := info.Compatible[:strings.LastIndex(info.Compatible, "/")+1] + "iframes/"
	playlist := string(speedTestGET(t, ctx, handler, base+"index.m3u8"))
	entries := speedTestURIs([]byte(playlist))
	if !strings.Contains(playlist, "#EXT-X-I-FRAMES-ONLY\n") || !strings.Contains(playlist, `#EXT-X-MAP:URI="init.mp4"`) || len(entries) != 60 || entries[30] != "300.m4s" || entries[59] != "590.m4s" || !strings.HasSuffix(playlist, "#EXT-X-ENDLIST\n") {
		t.Fatalf("I-frame playlist is wrong (%d entries):\n%.400s", len(entries), playlist)
	}
	frame := filepath.Join(t.TempDir(), "frame.mp4")
	if err := os.WriteFile(frame, append(speedTestGET(t, ctx, handler, base+"init.mp4"), speedTestGET(t, ctx, handler, base+"300.m4s")...), 0o600); err != nil {
		t.Fatal(err)
	}
	probe, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "v:0", "-show_entries", "frame=key_frame,pts_time,width,height", "-of", "csv=p=0", frame).Output()
	if err != nil || strings.Trim(string(probe), ", \n") != "1,300.000000,320,180" {
		t.Fatalf("I-frame fragment 300 decodes as %q (%v); want one 320x180 key frame at 300 s", probe, err)
	}
	matching, earlier := iframePSNR(t, ctx, ffmpeg, frame, source, 300), iframePSNR(t, ctx, ffmpeg, frame, source, 290)
	if matching < 35 || matching < earlier+15 {
		t.Fatalf("I-frame at 300 s matches source 300 s at %.1f dB and 290 s at %.1f dB", matching, earlier)
	}
	cached := iframeCacheFiles(t, cache)
	for _, name := range []string{"305.m4s", "600.m4s", "0010.m4s", "-10.m4s", "+10.m4s", "abc.m4s", "300.mp4", "index.m3u", "999990.m4s"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(ctx, http.MethodGet, base+name, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("I-frame %s = %d; want 404", name, response.Code)
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(ctx, http.MethodGet, strings.Replace(base, id, "ffffffffffffffff", 1)+"300.m4s", nil))
	if response.Code != http.StatusNotFound {
		t.Errorf("I-frame for an unknown item = %d; want 404", response.Code)
	}
	if after := iframeCacheFiles(t, cache); after != cached {
		t.Errorf("rejected I-frame requests changed the cache from %d to %d files", cached, after)
	}
}

func iframePSNR(t *testing.T, ctx context.Context, ffmpeg, frame, source string, second int) float64 {
	t.Helper()
	output, err := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-nostdin", "-i", frame, "-ss", strconv.Itoa(second), "-i", source,
		"-lavfi", "[0:v]setpts=0[frame];[1:v]trim=end_frame=1,scale=320:180,setsar=1,setpts=0[reference];[frame][reference]psnr", "-frames:v", "1", "-f", "null", "-").CombinedOutput()
	match := regexp.MustCompile(`PSNR .* average:([0-9.]+|inf)`).FindSubmatch(output)
	if err != nil || match == nil {
		t.Fatalf("compare I-frame with source %d s: %v: %s", second, err, output)
	}
	if string(match[1]) == "inf" {
		return 100
	}
	value, err := strconv.ParseFloat(string(match[1]), 64)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func iframeCacheFiles(t *testing.T, cache string) int {
	t.Helper()
	count := 0
	err := filepath.WalkDir(cache, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && bytes.Contains([]byte(path), []byte(string(filepath.Separator)+"iframes"+string(filepath.Separator))) {
			count++
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return count
}
