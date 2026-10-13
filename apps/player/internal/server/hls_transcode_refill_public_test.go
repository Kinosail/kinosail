package server_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/playback"
)

// A real HTTP refill must retain the clock of fragments already buffered by a
// client. Browser startup tests do not identify which encoder produced a fragment.
func TestRealTranscodedHLSRefillPreservesFrameClock(t *testing.T) {
	if os.Getenv("KINOSAIL_GO_TEST_MODE") != "deep" {
		t.Skip("The designated pinned-codec deep job runs this public media proof")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("hosted FFmpeg unavailable")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal("hosted FFprobe unavailable")
	}
	for _, size := range []string{"320x180", "1280x720"} {
		t.Run(size, func(t *testing.T) { transcodeRefillPublicClock(t, ffmpeg, ffprobe, size, false) })
		// AAC priming can affect a muxer's video clock. The video-only cases
		// cannot prove that the output policy is also stable with an audio track.
		t.Run(size+"-audio", func(t *testing.T) { transcodeRefillPublicClock(t, ffmpeg, ffprobe, size, true) })
	}
}

func transcodeRefillPublicClock(t *testing.T, ffmpeg, ffprobe, size string, audio bool) { //nolint:funlen,cyclop,gocognit // One ordered HTTP lifecycle binds original and regenerated media to joined producers.
	defer func() {
		transcodeRefillLog(t, map[string]any{"complete": true, "failed": t.Failed(), "revision": os.Getenv("GITHUB_SHA"), "os": runtime.GOOS, "arch": runtime.GOARCH, "go": runtime.Version(), "command": "KINOSAIL_GO_TEST_MODE=deep ./scripts/ci/test-go.sh player"})
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	source := filepath.Join(media, "Refill clock.mkv")
	arguments := []string{"-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=" + size + ":rate=25:duration=8"}
	if audio {
		arguments = append(arguments, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=8", "-c:a", "flac")
	}
	arguments = append(arguments, "-c:v", "ffv1", "-threads", "2", source)
	generate := exec.CommandContext(ctx, ffmpeg, arguments...) //nolint:gosec // Fixed bounded synthetic source and discovered hosted executable.
	if err := generate.Run(); err != nil {
		t.Fatal("eight-second FFV1 fixture generation failed")
	}
	sourceBefore := transcodeRefillRead(t, source)
	owned := filepath.Join(tools, "owned-pids")
	adapter := filepath.Join(tools, "ffmpeg")
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	script := "#!/bin/sh\nset -eu\nprintf '%s\\n' $$ >> " + quote(owned) + "\nprintf '%s\\0' \"$@\" > " + quote(tools) + "/args-$$\nexec " + quote(ffmpeg) + " \"$@\"\n"
	if err := os.WriteFile(adapter, []byte(script), 0o700); err != nil { //nolint:gosec // Owned exec-only adapter records provenance without changing codec arguments.
		t.Fatal(err)
	}
	handler, id := formatTestItem(t, server.Config{Lifecycle: ctx, MediaDir: media, CacheDir: cache, FFmpeg: adapter, FFprobe: ffprobe})
	host := httptest.NewServer(handler)
	defer host.Close()
	var info struct {
		Compatible     string
		CompatiblePlan playback.PlaybackPlan
	}
	if err := json.Unmarshal(bandwidthHTTPGET(t, ctx, host.URL+"/api/v1/items/"+id+"/playback?videoCodecs=h264&audioCodecs=aac"), &info); err != nil {
		t.Fatal(err)
	}
	if info.CompatiblePlan.Mode != "transcode" {
		t.Fatal("FFV1 fixture did not select actual video transcoding")
	}
	variants := speedTestURIs(bandwidthHTTPGET(t, ctx, host.URL+info.Compatible))
	if len(variants) == 0 || (size == "320x180" && len(variants) != 1) || (size == "1280x720" && len(variants) < 2) {
		t.Fatalf("fixture rendition admission failed: size=%s actual=%d", size, len(variants))
	}
	root := filepath.Join(cache, playback.HLSRecipeKey(id, playback.RecipeFor(info.CompatiblePlan)))
	for _, variant := range variants {
		for !bytes.Contains(transcodeRefillRead(t, filepath.Join(root, variant)), []byte("#EXT-X-ENDLIST")) {
			select {
			case <-ctx.Done():
				t.Fatal("initial presentation did not finish")
			case <-time.After(25 * time.Millisecond):
			}
		}
	}
	beforeOwned := copiedRecoveryJoinedCodecs(t, ctx, owned)
	prefix := host.URL + strings.TrimSuffix(info.Compatible, "index.m3u8")
	var originals [][][]byte
	for _, variant := range variants {
		base := prefix + strings.TrimSuffix(variant, "index.m3u8")
		var parts [][]byte
		for _, name := range []string{"init.mp4", "segment-00000.m4s", "segment-00001.m4s", "segment-00002.m4s"} {
			parts = append(parts, bandwidthHTTPGET(t, ctx, base+name))
		}
		originals = append(originals, parts)
	}
	// Evict one owned cache entry after its producer has exited. Public HTTP must
	// start a real refill. Forward files may be replaced by that presentation.
	if err := os.Remove(filepath.Join(root, filepath.Dir(variants[0]), "segment-00001.m4s")); err != nil {
		t.Fatal(err)
	}
	bandwidthHTTPGET(t, ctx, prefix+strings.TrimSuffix(variants[0], "index.m3u8")+"segment-00001.m4s")
	afterOwned := copiedRecoveryJoinedCodecs(t, ctx, owned)
	transcodeRefillLog(t, map[string]any{"revision": os.Getenv("GITHUB_SHA"), "size": size, "audio": audio, "renditions": len(variants), "initialProducers": beforeOwned, "totalProducers": afterOwned, "sourceSHA256": fmt.Sprintf("%x", sha256.Sum256(sourceBefore)), "producers": transcodeRefillProducers(t, tools)})
	if beforeOwned != 1 || afterOwned != 2 {
		t.Fatal("expected one joined initial presentation and one joined HTTP refill")
	}
	if !bytes.Equal(sourceBefore, transcodeRefillRead(t, source)) {
		t.Fatal("public refill mutated its source")
	}
	controls := make([]refillFrameTimeline, len(variants))
	for index, variant := range variants {
		t.Run(filepath.Dir(variant), func(t *testing.T) {
			base := prefix + strings.TrimSuffix(variant, "index.m3u8")
			original := originals[index]
			for part, name := range []string{"init.mp4", "segment-00000.m4s"} {
				if !bytes.Equal(original[part], bandwidthHTTPGET(t, ctx, base+name)) {
					t.Fatalf("refill changed retained %s", name)
				}
			}
			refilled := bandwidthHTTPGET(t, ctx, base+"segment-00001.m4s")
			// The last fragment is the original snapshot, as if already buffered.
			mixed := [][]byte{original[0], original[1], refilled, original[3]}
			control := transcodeRefillDecode(t, ctx, ffmpeg, "original", original)
			actual := transcodeRefillDecode(t, ctx, ffmpeg, "mixed", mixed)
			if len(control.Frames) != 150 || control.Numerator != 1 || control.Denominator < 25 || control.Denominator%25 != 0 {
				t.Fatal("control must decode exactly six seconds at 25 fps with an exact demux timebase")
			}
			step := control.Denominator / 25
			for frame, clock := range control.Frames {
				if clock.PTS != control.Frames[0].PTS+int64(frame)*step || clock.DTS != clock.PTS || clock.Duration != step {
					t.Fatalf("original control is not contiguous at frame %d", frame)
				}
			}
			if control.Numerator != actual.Numerator || control.Denominator != actual.Denominator || len(control.Frames) != len(actual.Frames) {
				t.Fatal("refill changed the decoded frame count or timebase")
			}
			for frame, clock := range control.Frames {
				if actual.Frames[frame] != clock {
					t.Fatalf("public refill shifted frame %d: original=%+v mixed=%+v", frame, clock, actual.Frames[frame])
				}
			}
			controls[index] = control
			if audio {
				transcodeRefillAudio(t, ctx, ffmpeg, "original", original)
				transcodeRefillAudio(t, ctx, ffmpeg, "mixed", mixed)
				transcodeRefillAudio(t, ctx, ffmpeg, "refilled-fragment", [][]byte{original[0], refilled})
			}
		})
	}
	if !t.Failed() {
		transcodeRefillLegacyCache(t, ctx, ffmpeg, host.URL+info.Compatible, prefix, root, owned, variants, controls, afterOwned, audio)
		if !bytes.Equal(sourceBefore, transcodeRefillRead(t, source)) {
			t.Fatal("legacy cache rebuild mutated its source")
		}
	}
}

func transcodeRefillRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("owned fixture or completed presentation file unavailable")
	}
	return data
}

func transcodeRefillLog(t *testing.T, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("transcode-refill-receipt %s", data)
	if directory := os.Getenv("KINOSAIL_TRANSCODE_REFILL_EVIDENCE"); directory != "" {
		transcodeRefillSave(t, directory, value)
	}
}
