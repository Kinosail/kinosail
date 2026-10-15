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
		t.Run(size, func(t *testing.T) { transcodeRefillPublicClock(t, ffmpeg, ffprobe, size, false, 1) })
		t.Run(size+"-audio", func(t *testing.T) { transcodeRefillPublicClock(t, ffmpeg, ffprobe, size, true, 1) })
	}
	t.Run("320x180-restarted-late-audio", func(t *testing.T) { transcodeRefillPublicClock(t, ffmpeg, ffprobe, "320x180", true, 7) })
}

func transcodeRefillPublicClock(t *testing.T, ffmpeg, ffprobe, size string, audio bool, targetSegment int) { //nolint:funlen,cyclop,gocognit // One ordered HTTP lifecycle binds original and regenerated media to joined producers.
	defer func() {
		command := "KINOSAIL_GO_TEST_MODE=deep ./scripts/ci/test-go.sh player"
		if os.Getenv("KINOSAIL_TRANSCODE_REFILL_PREFLIGHT") == "1" {
			command = "KINOSAIL_GO_TEST_MODE=deep go test -race -count=1 -run '^TestRealTranscodedHLSRefillPreservesFrameClock$/^320x180-restarted-late-audio$' ./internal/server"
		}
		transcodeRefillLog(t, map[string]any{"complete": true, "failed": t.Failed(), "revision": os.Getenv("GITHUB_SHA"), "os": runtime.GOOS, "arch": runtime.GOARCH, "go": runtime.Version(), "command": command})
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	source := filepath.Join(media, "Refill clock.mkv")
	duration := max(8, (targetSegment+3)*2)
	fragmentNames := []string{"init.mp4", fmt.Sprintf("segment-%05d.m4s", targetSegment-1), fmt.Sprintf("segment-%05d.m4s", targetSegment), fmt.Sprintf("segment-%05d.m4s", targetSegment+1)}
	arguments := []string{"-nostdin", "-v", "error", "-f", "lavfi", "-i", fmt.Sprintf("testsrc2=size=%s:rate=25:duration=%d", size, duration)}
	if audio {
		arguments = append(arguments, "-f", "lavfi", "-i", fmt.Sprintf("aevalsrc=0.25*sin(2*PI*(173*t+37*t*t))+0.1*sin(2*PI*(613*t+11*t*t)):s=48000:d=%d", duration), "-c:a", "flac")
	}
	arguments = append(arguments, "-c:v", "ffv1", "-threads", "2", source)
	generate := exec.CommandContext(ctx, ffmpeg, arguments...) //nolint:gosec // Fixed bounded synthetic source and discovered hosted executable.
	if err := generate.Run(); err != nil {
		t.Fatal("bounded FFV1 fixture generation failed")
	}
	sourceBefore := transcodeRefillRead(t, source)
	owned := filepath.Join(tools, "owned-pids")
	logs := captureCopiedLogs(t, 64<<10)
	defer func() { transcodeRefillDiagnostics(t, logs, tools) }()
	adapter := filepath.Join(tools, "ffmpeg")
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	script := "#!/bin/sh\nset -eu\nprintf '%s\\n' $$ >> " + quote(owned) + "\nprintf '%s\\0' \"$@\" > " + quote(tools) + "/args-$$\nexec " + quote(ffmpeg) + " \"$@\"\n"
	if err := os.WriteFile(adapter, []byte(script), 0o700); err != nil { //nolint:gosec // Owned exec-only adapter records provenance without changing codec arguments.
		t.Fatal(err)
	}
	firstLifecycle, stopFirst := context.WithCancel(ctx)
	defer stopFirst()
	config := server.Config{Lifecycle: firstLifecycle, MediaDir: media, CacheDir: cache, FFmpeg: adapter, FFprobe: ffprobe}
	handler, id := formatTestItem(t, config)
	host := httptest.NewServer(handler)
	defer func() { host.Close() }()
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
	if audio {
		for !bytes.Contains(bandwidthHTTPGET(t, ctx, host.URL+info.Compatible), []byte("#KINOSAIL-TRANSCODE-REFILL:")) {
			select {
			case <-ctx.Done():
				t.Fatal("completed presentation did not certify its public refill handoff")
			case <-time.After(25 * time.Millisecond):
			}
		}
	}
	prefix := host.URL + strings.TrimSuffix(info.Compatible, "index.m3u8")
	var originals [][][]byte
	for _, variant := range variants {
		base := prefix + strings.TrimSuffix(variant, "index.m3u8")
		var parts [][]byte
		for _, name := range fragmentNames {
			parts = append(parts, bandwidthHTTPGET(t, ctx, base+name))
		}
		originals = append(originals, parts)
	}
	var originalFirst [][]byte
	if targetSegment > 1 {
		base := prefix + strings.TrimSuffix(variants[0], "index.m3u8")
		for _, name := range []string{"init.mp4", "segment-00000.m4s", "segment-00001.m4s", "segment-00002.m4s"} {
			originalFirst = append(originalFirst, bandwidthHTTPGET(t, ctx, base+name))
		}
	}
	if targetSegment > 1 {
		stopFirst()
		host.Close()
		for _, variant := range variants {
			if err := os.Remove(filepath.Join(root, filepath.Dir(variant), "segment-00000.m4s")); err != nil {
				t.Fatal("cannot evict the owned initial fragment before restart")
			}
		}
		config.Lifecycle = ctx
		restarted, restartedID := formatTestItem(t, config)
		if restartedID != id {
			t.Fatal("restarted library changed the fixture identity")
		}
		host = httptest.NewServer(restarted)
		prefix = host.URL + strings.TrimSuffix(info.Compatible, "index.m3u8")
	}
	if err := os.Remove(filepath.Join(root, filepath.Dir(variants[0]), fragmentNames[2])); err != nil {
		t.Fatal(err)
	}
	bandwidthHTTPGET(t, ctx, prefix+strings.TrimSuffix(variants[0], "index.m3u8")+fragmentNames[2])
	afterOwned := copiedRecoveryJoinedCodecs(t, ctx, owned)
	transcodeRefillLog(t, map[string]any{"revision": os.Getenv("GITHUB_SHA"), "size": size, "audio": audio, "targetSegment": targetSegment, "restarted": targetSegment > 1, "renditions": len(variants), "initialProducers": beforeOwned, "totalProducers": afterOwned, "sourceSHA256": fmt.Sprintf("%x", sha256.Sum256(sourceBefore)), "producers": transcodeRefillProducers(t, tools)})
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
			if targetSegment > 1 {
				for _, part := range []int{0, 1, 3} {
					if !bytes.Equal(original[part], transcodeRefillRead(t, filepath.Join(root, filepath.Dir(variant), fragmentNames[part]))) {
						t.Fatalf("restarted refill changed retained cache file %s", fragmentNames[part])
					}
				}
			}
			for part, name := range fragmentNames[:2] {
				if !bytes.Equal(original[part], bandwidthHTTPGET(t, ctx, base+name)) {
					t.Fatalf("refill changed retained %s", name)
				}
			}
			if targetSegment > 1 {
				if !bytes.Equal(original[3], bandwidthHTTPGET(t, ctx, base+fragmentNames[3])) {
					t.Error("restarted refill replaced its retained later neighbor")
				}
			}
			refilled := bandwidthHTTPGET(t, ctx, base+fragmentNames[2])
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
				originalFragment := transcodeRefillAudioPackets(t, ctx, ffprobe, "original-fragment", [][]byte{original[0], original[2]})
				originalPCM, originalAudio := transcodeRefillAudio(t, ctx, ffmpeg, ffprobe, "original", original)
				mixedPCM, mixedAudio := transcodeRefillAudio(t, ctx, ffmpeg, ffprobe, "mixed", mixed)
				_, refilledAudio := transcodeRefillAudio(t, ctx, ffmpeg, ffprobe, "refilled-fragment", [][]byte{original[0], refilled})
				transcodeRefillCompareAudio(t, "mixed", originalAudio, mixedAudio)
				transcodeRefillCompareAudio(t, "refilled-fragment", originalFragment, refilledAudio)
				if len(originalPCM) != len(mixedPCM) {
					t.Errorf("mixed fragments changed the decoded PCM length: original=%d actual=%d", len(originalPCM), len(mixedPCM))
				}
				transcodeRefillContent(t, originalPCM, mixedPCM, originalAudio, originalFragment)
			}
		})
	}
	if targetSegment > 1 {
		finalOwned := copiedRecoveryJoinedCodecs(t, ctx, owned)
		transcodeRefillLog(t, map[string]any{"finalProducers": finalOwned, "targetSegment": targetSegment})
		if finalOwned != afterOwned || finalOwned != 2 {
			t.Error("retained fragment reads triggered an additional producer")
		}
		transcodeRefillBoundedInputs(t, tools, targetSegment)
		for _, variant := range variants {
			_, zeroErr := os.Stat(filepath.Join(root, filepath.Dir(variant), "segment-00000.m4s"))
			transcodeRefillLog(t, map[string]any{"restartedWithoutZero": true, "zeroRemainsMissing": os.IsNotExist(zeroErr)})
			if !os.IsNotExist(zeroErr) {
				t.Error("late refill rebuilt the missing first fragment")
			}
		}
		if !bytes.Equal(sourceBefore, transcodeRefillRead(t, source)) {
			t.Error("final restarted requests mutated their source")
		}
		if !t.Failed() {
			transcodeRefillRejectDamagedProof(t, ctx, root, prefix, owned, variants[0], fragmentNames[2], source, finalOwned)
			finalOwned = transcodeRefillRejectClockMismatch(t, ctx, root, prefix, owned, variants[0], fragmentNames[2], source, finalOwned, logs)
			finalOwned = transcodeRefillRejectShortAudio(t, ctx, root, prefix, owned, variants[0], fragmentNames[2], source, adapter, finalOwned, logs)
			transcodeRefillRecoverFirst(t, ctx, ffmpeg, ffprobe, prefix+strings.TrimSuffix(variants[0], "index.m3u8"), owned, originalFirst, finalOwned)
		}
	}
	if !t.Failed() && targetSegment == 1 {
		transcodeRefillLegacyCache(t, ctx, ffmpeg, ffprobe, host.URL+info.Compatible, prefix, root, owned, variants, controls, afterOwned, audio)
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
