package server_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/playback"
)

func TestRealCopiedVideoHLSSegmentsDecodeIndependently(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for the HLS random-access integration test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required for the HLS random-access integration test")
	}
	for _, audio := range []string{"aac", "ac3"} {
		t.Run(audio, func(t *testing.T) {
			assertCopiedHLSAudio(t, ffmpeg, ffprobe, audio)
		})
	}
}

func assertCopiedHLSAudio(t *testing.T, ffmpeg, ffprobe, audio string) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	media := independentHLSMedia(t, ctx, ffmpeg, audio)
	logs := captureCopiedLogs(t)
	cache := t.TempDir()
	retainCopiedFailureFacts(t, ffmpeg, ffprobe, filepath.Join(media, "Episode.S01E01.mkv"), cache, logs)
	retainCopiedPlaylistFacts(t, cache)
	handler, id := formatTestItem(t, server.Config{Lifecycle: ctx, MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe})
	var info struct {
		Compatible     string
		CompatiblePlan playback.PlaybackPlan
	}
	mustJSON(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback?videoCodecs=h264&audioCodecs=aac", nil), &info)
	mode := "remux"
	if audio == "ac3" {
		mode = "audio-transcode"
	}
	if info.CompatiblePlan.Mode != mode {
		t.Fatalf("fixture plan = %s; want %s", info.CompatiblePlan.Mode, mode)
	}
	for _, offset := range []int{0, 12700} {
		t.Run(fmt.Sprintf("offset-%d", offset), func(t *testing.T) {
			source := info.Compatible
			if offset != 0 {
				source = strings.Replace(source, "/index.m3u8", fmt.Sprintf("-o%d/index.m3u8", offset), 1)
			}
			master := speedTestGET(t, ctx, handler, source)
			base := source[:strings.LastIndex(source, "/")+1]
			variants := speedTestURIs(master)
			if len(variants) == 0 {
				t.Fatal("HLS master has no video renditions")
			}
			for _, variant := range variants {
				assertIndependentHLSRendition(t, ctx, handler, base+variant, ffmpeg, ffprobe)
			}
		})
	}
}

func independentHLSMedia(t *testing.T, ctx context.Context, ffmpeg, audio string) string {
	t.Helper()
	media := t.TempDir()
	command := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24:duration=36", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=36", "-c:v", "libx264", "-preset", "veryfast", "-g", "120", "-keyint_min", "120", "-sc_threshold", "0", "-c:a", audio, filepath.Join(media, "Episode.S01E01.mkv")) //nolint:gosec // Fixed synthetic media and discovered local tools.
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate long-GOP fixture: %v: %s", err, output)
	}
	return media
}

func assertIndependentHLSRendition(t *testing.T, ctx context.Context, handler http.Handler, source, ffmpeg, ffprobe string) {
	t.Helper()
	playlist := speedTestGET(t, ctx, handler, source)
	deadline := time.Now().Add(10 * time.Second)
	for !bytes.Contains(playlist, []byte("#EXT-X-ENDLIST")) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		playlist = speedTestGET(t, ctx, handler, source)
	}
	if !bytes.Contains(playlist, []byte("#EXT-X-ENDLIST")) {
		t.Fatal("HLS fixture did not complete")
	}
	base := source[:strings.LastIndex(source, "/")+1]
	initialization := speedTestGET(t, ctx, handler, base+"init.mp4")
	segments := speedTestURIs(playlist)
	if len(segments) < 3 {
		t.Fatalf("fixture needs multiple random-access segments: %d", len(segments))
	}
	for _, segment := range segments {
		fragment := append(bytes.Clone(initialization), speedTestGET(t, ctx, handler, base+segment)...)
		assertIndependentHLSFragment(t, ctx, fragment, segment, ffmpeg, ffprobe)
	}
}

func assertIndependentHLSFragment(t *testing.T, ctx context.Context, fragment []byte, segment, ffmpeg, ffprobe string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fragment.mp4")
	if err := os.WriteFile(path, fragment, 0o600); err != nil {
		t.Fatal(err)
	}
	probe := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "v:0", "-show_entries", "packet=flags", "-of", "csv=p=0", path) //nolint:gosec // Generated fixture paths and discovered local tools.
	packets, err := probe.CombinedOutput()
	first := strings.SplitN(string(packets), "\n", 2)[0]
	if err != nil || !strings.Contains(first, "K") {
		t.Errorf("%s first video packet is not a keyframe: %q (%v)", segment, first, err)
		return
	}
	// A partial segment's inferred frame rate must not round distinct timestamps together.
	decode := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-err_detect", "explode", "-i", path, "-map", "0:v:0", "-enc_time_base", "1:1000000", "-fps_mode", "passthrough", "-f", "rawvideo", "-") //nolint:gosec // Generated fixture paths and discovered local tools.
	var errors bytes.Buffer
	decode.Stdout, decode.Stderr = io.Discard, &errors
	if err := decode.Run(); err != nil || errors.Len() != 0 {
		t.Errorf("%s does not decode independently: %v: %s", segment, err, errors.String())
	}
}
