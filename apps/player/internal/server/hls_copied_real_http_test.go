package server_test

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/playback"
)

// This real codec journey covers speculative indexing through its public POST,
// unlike the existing independent-decode GET-only owner. It verifies admission,
// full source duration, delivery of every certified cut and decoded media.
func TestRealCopiedHLSHTTPPreparationDeliversEveryCertifiedFragment(t *testing.T) {
	for _, audio := range []bool{false, true} {
		name := "silent fixed IDR"
		if audio {
			name = "AAC B-frame certified cuts"
		}
		t.Run(name, func(t *testing.T) { realCopiedHLSHTTP(t, audio) })
	}
}

func realCopiedHLSHTTP(t *testing.T, audio bool) {
	t.Helper()
	ffmpeg, ffprobe := copiedHLSTools(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	media := generateCopiedHLSMedia(t, ctx, ffmpeg, audio)
	logs := captureCopiedLogs(t)
	cache := t.TempDir()
	retainCopiedFailureFacts(t, ffmpeg, ffprobe, filepath.Join(media, "Indexed.mkv"), cache, logs)
	config := server.Config{Lifecycle: ctx, MediaDir: media, DataDir: t.TempDir(), CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe}
	h, id := formatTestItem(t, config)
	var plan struct {
		Compatible     string
		CompatiblePlan playback.PlaybackPlan
	}
	mustJSON(t, apiCall(t, h, "", http.MethodGet, "/api/v1/items/"+id+"/playback?videoCodecs=h264&audioCodecs=aac", nil), &plan)
	if plan.CompatiblePlan.Mode != "remux" {
		t.Fatalf("copied plan = %s", plan.CompatiblePlan.Mode)
	}
	awaitCopiedReady(t, copiedHTTPFixture{handler: h, id: id, source: plan.Compatible})
	master := speedTestGET(t, ctx, h, plan.Compatible)
	variants := speedTestURIs(master)
	if len(variants) != 1 {
		t.Fatalf("copied rendition count = %d", len(variants))
	}
	base := plan.Compatible[:strings.LastIndex(plan.Compatible, "/")+1] + strings.TrimSuffix(variants[0], "index.m3u8")
	playlist := speedTestGET(t, ctx, h, base+"index.m3u8")
	if !bytes.Contains(playlist, []byte("#EXT-X-PLAYLIST-TYPE:VOD")) || !bytes.Contains(playlist, []byte("#EXT-X-ENDLIST")) {
		t.Fatalf("prepared public timeline is incomplete: %s", playlist)
	}
	segments := speedTestURIs(playlist)
	if len(segments) != 6 {
		t.Fatalf("12-second source expected 6 safe cuts, got %d", len(segments))
	}
	initialization := speedTestGET(t, ctx, h, base+"init.mp4")
	complete := bytes.Clone(initialization)
	for _, segment := range segments {
		data := speedTestGET(t, ctx, h, base+segment)
		complete = append(complete, data...)
		fragment := append(bytes.Clone(initialization), data...)
		assertIndependentHLSFragment(t, ctx, fragment, segment, ffmpeg, ffprobe)
	}
	assertCopiedHLSContent(t, t.Context(), ffmpeg, filepath.Join(media, "Indexed.mkv"), complete, audio)
	// Finish the original worker so each public refill has independent ownership.
	cancel()
	cuts := []int{4}
	if audio {
		cuts = []int{1, 2, 3, 4, 5}
	}
	for _, cut := range cuts {
		refillCopiedHLSMedia(t, config, ffmpeg, ffprobe, base, initialization, segments, complete, audio, cut)
	}
}

func generateCopiedHLSMedia(t *testing.T, ctx context.Context, ffmpeg string, audio bool) string {
	t.Helper()
	media := t.TempDir()
	// Silent, fixed two-second IDR groups start at zero; demux and decoder clocks
	// must still be measured by the production pipeline, not supplied by a fixture.
	args := []string{"-v", "error", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=12:duration=12"}
	if audio {
		args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:duration=12")
	}
	args = append(args, "-c:v", "libx264", "-preset", "ultrafast", "-g", "24", "-keyint_min", "24", "-sc_threshold", "0")
	if audio {
		args = append(args, "-bf", "2", "-c:a", "aac", "-shortest")
	} else {
		args = append(args, "-bf", "0", "-an")
	}
	args = append(args, filepath.Join(media, "Indexed.mkv"))
	generate := exec.CommandContext(ctx, ffmpeg, args...)               //nolint:gosec // Fixed generated fixture and discovered local executable.
	version, _ := exec.CommandContext(ctx, ffmpeg, "-version").Output() //nolint:gosec // Discovered local executable and fixed arguments.
	t.Logf("real encoder: %s", strings.Split(string(version), "\n")[0])
	if output, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("generate copied fixture: %v: %s", err, output)
	}

	return media
}

func refillCopiedHLSMedia(t *testing.T, config server.Config, ffmpeg, ffprobe, base string, initialization []byte, segments []string, initialMedia []byte, audio bool, cut int) {
	t.Helper()
	refillContext, refillCancel := context.WithCancel(t.Context())
	t.Cleanup(refillCancel)
	config.Lifecycle = refillContext
	h := server.New(config)
	target := ""
	if err := filepath.WalkDir(config.CacheDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Name() == segments[cut] {
			target = path
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if target == "" {
		t.Fatal("publicly delivered refill target missing")
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	refilledInitialization := speedTestGET(t, t.Context(), h, base+"init.mp4")
	if !bytes.Equal(initialization, refilledInitialization) {
		t.Fatal("valid cached initialization changed while a different fragment was evicted")
	}
	complete := bytes.Clone(refilledInitialization)
	for _, segment := range segments {
		data := speedTestGET(t, t.Context(), h, base+segment)
		complete = append(complete, data...)
		assertIndependentHLSFragment(t, t.Context(), append(bytes.Clone(initialization), data...), segment, ffmpeg, ffprobe)
	}
	refillCancel()
	t.Logf("public eviction/refill certified cut %d", cut)
	assertCopiedHLSContent(t, t.Context(), ffmpeg, filepath.Join(config.MediaDir, "Indexed.mkv"), complete, audio)
	assertCopiedRefillDecode(t, t.Context(), ffmpeg, initialMedia, complete, audio)
}

func copiedHLSTools(t *testing.T) (string, string) {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for real copied-HLS preparation")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required for real copied-HLS preparation")
	}
	return ffmpeg, ffprobe
}
