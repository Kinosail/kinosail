package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/workload"
)

func TestRemainingNonKeyActualSourceClock(t *testing.T) {
	if os.Getenv("KINOSAIL_COPIED_RECOVERY_MEDIA") != "1" {
		t.Skip("The pinned hosted job owns the real source-clock operation")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("source-clock pinned FFmpeg missing")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal("source-clock pinned FFprobe missing")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	sources := remainingNonKeyCollectorSources(t, ctx, ffmpeg)
	for _, source := range sources {
		for _, offset := range []float64{12.5, 13.5, 18.2} {
			t.Run(fmt.Sprintf("%s-%.1f", filepath.Ext(source), offset), func(t *testing.T) {
				remainingNonKeyActualClock(t, ctx, ffmpeg, ffprobe, source, offset)
			})
		}
	}
}

func remainingNonKeyActualClock(t *testing.T, ctx context.Context, ffmpeg, ffprobe, source string, offset float64) {
	t.Helper()
	first, original := remainingNonKeyCollectorPrivateAudio(t, ctx, ffmpeg, ffprobe, source, offset)
	expectedHash, expected := remainingNonKeyCollectorExpected(t, source, offset)
	if hex.EncodeToString(first[:]) != expectedHash || original != expected[8] {
		t.Fatal("nonkey actual private fixture differs from independently measured packet and edit")
	}
	manager, item, _, _ := hlsLoadingFixtureContext(t, ctx)
	item.Path = source
	manager.index = &libraryIndex{Index: catalog.NewMemoryIndex(nil, true)}
	manager.index.SetRoots([]catalog.ScanRoot{{Path: filepath.Dir(source)}})
	manager.probe, manager.ffmpeg = newMediaProbe(ffprobe), ffmpeg
	recipe := hlsRecipe{mode: "remux", offset: offset}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		t.Fatal("source-clock initial policy")
	}
	releaseWork, err := manager.workloads.Acquire(ctx, workload.Playback)
	if err != nil {
		t.Fatal("source-clock owner reservation")
	}
	defer releaseWork()
	remainingNonKeyCollectorReserved(t, manager)
	before := remainingNonKeyCollectorHash(t, source)
	started := time.Now()
	proof, err := manager.measureCopiedHLSSourceAudio(ctx, item, recipe, options.Cache, first, int64(offset*1_000_000), original)
	elapsed := time.Since(started)
	remainingNonKeyCollectorResult(t, proof, err, elapsed, first, offset, original)
	if remainingNonKeyCollectorHash(t, source) != before {
		t.Fatal("nonkey source-clock producer changed its source")
	}
	remainingNonKeyCollectorCorrespondence(t, proof, expected)
	remainingNonKeyCollectorReserved(t, manager)
	remainingNonKeyCollectorCacheEmpty(t, manager)
	t.Logf("nonkey actual-clock container=%s offset=%.1f elapsed_ns=%d source_clock=%x first_packet=%x first_native=%d target_native=%d target_pts=%d media_time=%d original_media_time=%d leading=%d phase=%d",
		filepath.Ext(source), offset, elapsed.Nanoseconds(), proof.SourceClock, proof.FirstPacket,
		proof.FirstNativeSample, proof.TargetNativeSample, proof.TargetPTS, proof.MediaTime,
		proof.OriginalMediaTime, proof.LeadingSamples, proof.SourcePhase)
	remainingNonKeyCollectorRejects(t, ctx, manager, item, recipe, first, original, options.Cache)
}

func remainingNonKeyCollectorResult(t *testing.T, proof *copiedHLSAudioProof, err error, elapsed time.Duration, first [32]byte, offset float64, original int64) {
	t.Helper()
	if err != nil || proof == nil {
		t.Fatal("nonkey actual source-clock producer missing or rejected qualified private packet")
	}
	if elapsed > 2*time.Second || proof.FirstPacket != first || proof.SourceClock == [32]byte{} ||
		proof.RequestedSample != int64(offset*48000) || proof.OriginalMediaTime != original {
		t.Fatal("nonkey actual source-clock identity or complete-operation budget")
	}
}

func remainingNonKeyCollectorRejects(t *testing.T, ctx context.Context, manager *hlsManager, item library.Item, recipe hlsRecipe, first [32]byte, original int64, policy string) {
	t.Helper()
	before := remainingNonKeyCollectorHash(t, item.Path)
	for _, damage := range []string{"absent-packet", "canceled", "changed-policy"} {
		t.Run(damage, func(t *testing.T) {
			request, release := context.WithCancel(ctx)
			defer release()
			packet, currentPolicy := first, policy
			switch damage {
			case "absent-packet":
				packet = [32]byte{1}
			case "canceled":
				release()
			case "changed-policy":
				currentPolicy += ":changed"
			}
			if value, err := manager.measureCopiedHLSSourceAudio(request, item, recipe, currentPolicy, packet, int64(recipe.offset*1_000_000), original); err == nil || value != nil {
				t.Fatal("nonkey invalid source-clock operation produced a proof")
			}
			if remainingNonKeyCollectorHash(t, item.Path) != before {
				t.Fatal("nonkey rejected source-clock operation changed source")
			}
		})
	}
}

// The measured packet is obtained from actual generated private assets, never
// from an inferred source ordinal. This does not certify production BMFF parsing.
func remainingNonKeyCollectorPrivateAudio(t *testing.T, ctx context.Context, ffmpeg, ffprobe, source string, offset float64) ([32]byte, int64) {
	t.Helper()
	directory := remainingNonKeyCollectorGeneratePrivate(t, ctx, ffmpeg, source, offset)
	initialization := remainingNonKeyCollectorRead(t, filepath.Join(directory, "init.mp4"), 2<<20)
	fragment := remainingNonKeyCollectorRead(t, filepath.Join(directory, "segment-00000.m4s"), 8<<20)
	joined := filepath.Join(directory, "private-first.mp4")
	if os.WriteFile(joined, append(append([]byte{}, initialization...), fragment...), 0o600) != nil {
		t.Fatal("source-clock owned private join")
	}
	output := remainingNonKeyCollectorCommand(t, ctx, ffprobe, "-v", "error", "-select_streams", "a:0",
		"-show_packets", "-show_data_hash", "sha256", "-show_entries", "packet=data_hash", "-of", "json", joined)
	var facts struct {
		Packets []struct {
			Hash string `json:"data_hash"`
		} `json:"packets"`
	}
	if json.Unmarshal(output, &facts) != nil || len(facts.Packets) == 0 {
		t.Fatal("source-clock actual private audio packets missing")
	}
	first := copiedHLSSourceAudioHash(facts.Packets[0].Hash)
	if first == [32]byte{} {
		t.Fatal("source-clock actual private packet identity")
	}
	original := remainingNonKeyCollectorAudioEdit(t, initialization)
	t.Logf("nonkey actual-clock-input container=%s offset=%.1f source_sha=%x private_init_sha=%x private_first_sha=%x first_aac_sha=%x original_edit=%d",
		filepath.Ext(source), offset, remainingNonKeyCollectorHash(t, source), sha256.Sum256(initialization), sha256.Sum256(fragment), first, original)
	return first, original
}

func remainingNonKeyCollectorGeneratePrivate(t *testing.T, ctx context.Context, ffmpeg, source string, offset float64) string {
	t.Helper()
	directory := t.TempDir()
	remainingNonKeyCollectorCommand(t, ctx, ffmpeg, "-nostdin", "-v", "error", "-y", "-ss", fmt.Sprint(offset),
		"-i", source, "-map", "0:v:0", "-map", "0:a:0?", "-sn", "-c", "copy", "-avoid_negative_ts", "disabled",
		"-f", "hls", "-hls_time", "2", "-hls_playlist_type", "event", "-hls_segment_type", "fmp4",
		"-hls_segment_options", "movflags=+skip_sidx:avoid_negative_ts=disabled",
		"-hls_flags", "temp_file", "-hls_fmp4_init_filename", "init.mp4",
		"-hls_segment_filename", filepath.Join(directory, "segment-%05d.m4s"), filepath.Join(directory, "index.m3u8"))
	return directory
}

func remainingNonKeyCollectorSources(t *testing.T, ctx context.Context, ffmpeg string) []string {
	t.Helper()
	directory := t.TempDir()
	mkv, mp4 := filepath.Join(directory, "Clock.mkv"), filepath.Join(directory, "Clock.mp4")
	remainingNonKeyCollectorCommand(t, ctx, ffmpeg, "-nostdin", "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=s=640x360:r=24:d=32.0",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=32.0",
		"-c:v", "libx264", "-threads", "2", "-preset", "veryfast", "-crf", "32",
		"-pix_fmt", "yuv420p", "-g", "48", "-keyint_min", "1", "-sc_threshold", "0",
		"-force_key_frames", "0,2,4,6,8,10,12,14,16,18,20,22,24,26,28,30", "-frames:v", "768",
		"-c:a", "aac", "-ac", "2", mkv)
	remainingNonKeyCollectorCommand(t, ctx, ffmpeg, "-nostdin", "-v", "error", "-i", mkv,
		"-map", "0:v:0", "-map", "0:a:0", "-c", "copy", mp4)
	return []string{mkv, mp4}
}

func remainingNonKeyCollectorCommand(t *testing.T, ctx context.Context, executable string, arguments ...string) []byte {
	t.Helper()
	var output strings.Builder
	err := copiedHLSLines(ctx, executable, arguments, 2<<20, 24_000, func(line string) error {
		_, _ = output.WriteString(line)
		return output.WriteByte('\n')
	})
	if err != nil {
		t.Fatal("source-clock owned fixture command or output bound")
	}
	return []byte(output.String())
}

func remainingNonKeyCollectorRead(t *testing.T, name string, maximum int) []byte {
	t.Helper()
	info, err := os.Stat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > int64(maximum) {
		t.Fatal("source-clock owned fixture file bound")
	}
	data, err := os.ReadFile(name)
	if err != nil || len(data) != int(info.Size()) {
		t.Fatal("source-clock owned fixture read")
	}
	return data
}

func remainingNonKeyCollectorHash(t *testing.T, name string) [32]byte {
	t.Helper()
	return sha256.Sum256(remainingNonKeyCollectorRead(t, name, 64<<20))
}

func remainingNonKeyCollectorReserved(t *testing.T, manager *hlsManager) {
	t.Helper()
	metrics := manager.workloads.Metrics()
	if metrics.Capacity != 1 || metrics.ActivePlayback != 1 || metrics.ActiveBackground != 0 ||
		metrics.WaitingPlayback != 0 || metrics.WaitingBackground != 0 {
		t.Fatal("nonkey source-clock nested or released the owner reservation")
	}
}

func remainingNonKeyCollectorCacheEmpty(t *testing.T, manager *hlsManager) {
	t.Helper()
	entries, err := os.ReadDir(manager.cache)
	if err != nil || len(entries) != 0 || len(manager.jobs) != 0 {
		t.Fatal("nonkey source-clock measurement exposed cache output")
	}
}
