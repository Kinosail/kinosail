package server_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/playback"
)

// Actual public preparation must distinguish a requested presentation origin
// from its preceding independent decode key. Cold fallback does not prove this.
func TestRemainingNonKeyPreparationReachesReady(t *testing.T) {
	if os.Getenv("KINOSAIL_COPIED_RECOVERY_MEDIA") != "1" {
		t.Skip("The designated pinned-codec hosted job runs this public regression")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("nonkey fixture: pinned FFmpeg unavailable")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal("nonkey fixture: pinned FFprobe unavailable")
	}
	for _, offset := range []float64{12, 12.5} {
		t.Run(fmt.Sprintf("offset-%.1f", offset), func(t *testing.T) {
			remainingNonKeyPreparation(t, ffmpeg, ffprobe, offset)
		})
	}
}

func remainingNonKeyPreparation(t *testing.T, ffmpeg, ffprobe string, offset float64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	media := t.TempDir()
	source := filepath.Join(media, "Nonkey.mp4")
	command := exec.CommandContext(ctx, ffmpeg, "-nostdin", "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=s=640x360:r=24:d=32",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=32",
		"-c:v", "libx264", "-threads", "2", "-preset", "veryfast", "-crf", "32",
		"-g", "48", "-keyint_min", "48", "-sc_threshold", "0", "-frames:v", "768",
		"-c:a", "aac", "-ac", "2", "-avoid_negative_ts", "disabled", source) //nolint:gosec // Fixed owned fixture and discovered pinned codec.
	if command.Run() != nil {
		t.Fatal("nonkey fixture generation failed")
	}
	remainingNonKeyQualify(t, ctx, ffmpeg, ffprobe, source)
	before, err := os.ReadFile(source)
	if err != nil || len(before) == 0 || len(before) > 64<<20 {
		t.Fatal("nonkey fixture byte bound")
	}
	tools, cache := t.TempDir(), t.TempDir()
	owned, adapter := filepath.Join(tools, "owned-pids"), filepath.Join(tools, "ffmpeg")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	body := "#!/bin/sh\nset -eu\nprintf '%s\\n' $$ >> " + quote(owned) + "\nexec " + quote(ffmpeg) + " \"$@\"\n"
	//nolint:gosec // Owned executable adapter.
	if os.WriteFile(adapter, []byte(body), 0o700) != nil {
		t.Fatal("nonkey owned codec adapter")
	}
	handler, id := formatTestItem(t, server.Config{Lifecycle: ctx, MediaDir: media,
		DataDir: t.TempDir(), CacheDir: cache, FFmpeg: adapter, FFprobe: ffprobe})
	t.Cleanup(func() {
		cancel()
		settled, release := context.WithTimeout(context.Background(), 3*time.Second)
		defer release()
		if data, readErr := os.ReadFile(owned); readErr == nil && len(data) > 0 {
			copiedRecoveryJoinedCodecs(t, settled, owned)
		}
	})
	var plan struct {
		Compatible     string
		CompatiblePlan playback.PlaybackPlan
	}
	mustJSON(t, apiCall(t, handler, "", http.MethodGet,
		"/api/v1/items/"+id+"/playback?videoCodecs=h264&audioCodecs=aac", nil), &plan)
	if plan.CompatiblePlan.Mode != "remux" || !strings.HasSuffix(plan.Compatible, "/index.m3u8") {
		t.Fatal("nonkey public recipe qualification")
	}
	t.Logf("nonkey fixture qualified offset=%.1f sha=%x frames=768 idr-keys=16 eof=32", offset, sha256.Sum256(before))
	selected := strings.TrimSuffix(plan.Compatible, "/index.m3u8") +
		fmt.Sprintf("-o%d/index.m3u8", int(offset*1000))
	deadline := time.Now().Add(15 * time.Second)
	state := ""
	for {
		var preparation struct{ State string }
		response := apiCall(t, handler, "", http.MethodPost,
			"/api/v1/items/"+id+"/playback-prepare", map[string]any{"source": selected})
		if response.Code != http.StatusAccepted {
			t.Fatalf("nonkey preparation status=%d", response.Code)
		}
		mustJSON(t, response, &preparation)
		state = preparation.State
		if state == "ready" {
			break
		}
		if state != "queued" {
			t.Fatalf("nonkey regression offset=%.1f phase=public-preparation state=%s expected=ready fixture=%x", offset, state, sha256.Sum256(before))
		}
		if time.Now().After(deadline) {
			t.Fatalf("nonkey preparation deadline offset=%.1f state=%s", offset, state)
		}
		time.Sleep(20 * time.Millisecond)
	}
	after, err := os.ReadFile(source)
	if err != nil || sha256.Sum256(after) != sha256.Sum256(before) {
		t.Fatal("nonkey preparation mutated its source")
	}
	t.Logf("nonkey regression offset=%.1f phase=public-preparation state=%s fixture=%x frames=768 idr-keys=16", offset, state, sha256.Sum256(before))
}

func remainingNonKeyQualify(t *testing.T, ctx context.Context, ffmpeg, ffprobe, source string) {
	t.Helper()
	output, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-count_frames",
		"-show_entries", "stream=codec_name,codec_type,nb_read_frames,channels,sample_rate:format=duration",
		"-of", "json", source).Output() //nolint:gosec // Owned bounded fixture and pinned probe.
	if err != nil || len(output) > 32<<10 {
		t.Fatal("nonkey fixture stream probe bound")
	}
	var facts struct {
		Streams []struct {
			Codec    string `json:"codec_name"`
			Kind     string `json:"codec_type"`
			Frames   string `json:"nb_read_frames"`
			Channels int `json:"channels"`
			Rate     string `json:"sample_rate"`
		}
		Format struct{ Duration string }
	}
	if json.Unmarshal(output, &facts) != nil {
		t.Fatal("nonkey fixture stream JSON")
	}
	video, audio := false, false
	for _, stream := range facts.Streams {
		video = video || stream.Kind == "video" && stream.Codec == "h264" && stream.Frames == "768"
		audio = audio || stream.Kind == "audio" && stream.Codec == "aac" && stream.Channels == 2 && stream.Rate == "48000"
	}
	duration, err := strconv.ParseFloat(facts.Format.Duration, 64)
	if !video || !audio || err != nil || duration < 31.95 || duration > 32.05 {
		t.Fatal("nonkey fixture codec/frame/EOF qualification")
	}
	output, err = exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "v:0",
		"-show_packets", "-show_entries", "packet=pts_time,flags", "-of", "json", source).Output() //nolint:gosec // Owned bounded fixture and pinned probe.
	if err != nil || len(output) > 512<<10 {
		t.Fatal("nonkey fixture packet probe bound")
	}
	var packets struct {
		Packets []struct {
			PTS   string `json:"pts_time"`
			Flags string `json:"flags"`
		}
	}
	if json.Unmarshal(output, &packets) != nil || len(packets.Packets) != 768 {
		t.Fatal("nonkey fixture complete video packet rows")
	}
	keys := 0
	for _, packet := range packets.Packets {
		if !strings.Contains(packet.Flags, "K") {
			continue
		}
		point, err := strconv.ParseFloat(packet.PTS, 64)
		if err != nil || point != float64(keys*2) {
			t.Fatal("nonkey fixture independent two-second source keys")
		}
		keys++
	}
	if keys != 16 {
		t.Fatal("nonkey fixture independent key count")
	}
	remainingNonKeyIDRs(t, ctx, ffmpeg, source)
}

func remainingNonKeyIDRs(t *testing.T, ctx context.Context, ffmpeg, source string) {
	t.Helper()
	output, err := exec.CommandContext(ctx, ffmpeg, "-nostdin", "-v", "error",
		"-threads", "1", "-copyts", "-i", source, "-map", "0:v:0", "-an", "-sn", "-dn",
		"-c:v", "copy", "-copytb", "1", "-bsf:v", "filter_units=pass_types=5",
		"-f", "framehash", "pipe:1").Output() //nolint:gosec // Fixed IDR filter on owned bounded fixture.
	if err != nil || len(output) > 32<<10 {
		t.Fatal("nonkey fixture IDR certification bound")
	}
	base, count := float64(0), 0
	for _, line := range strings.Split(string(output), "\n") {
		if value, ok := strings.CutPrefix(line, "#tb 0: "); ok {
			a, b, found := strings.Cut(value, "/")
			numerator, aErr := strconv.ParseFloat(a, 64)
			denominator, bErr := strconv.ParseFloat(b, 64)
			if !found || aErr != nil || bErr != nil || numerator <= 0 || denominator <= 0 {
				t.Fatal("nonkey fixture IDR time base")
			}
			base = numerator / denominator
			continue
		}
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) != 6 || count >= 16 || base <= 0 {
			t.Fatal("nonkey fixture IDR packet structure")
		}
		pts, ptsErr := strconv.ParseFloat(strings.TrimSpace(fields[2]), 64)
		size, sizeErr := strconv.Atoi(strings.TrimSpace(fields[4]))
		if ptsErr != nil || sizeErr != nil || size <= 0 || pts*base != float64(count*2) {
			t.Fatal("nonkey fixture IDR correspondence")
		}
		count++
	}
	if count != 16 {
		t.Fatal("nonkey fixture complete IDR correspondence")
	}
}
