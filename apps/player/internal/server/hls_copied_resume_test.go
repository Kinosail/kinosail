package server_test

import (
	"bytes"
	"context"
	"fmt"
	"math"
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

func TestRealCopiedVideoResumeSegmentsMatchPlaylist(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for the HLS resume integration test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required for the HLS resume integration test")
	}
	for _, fixture := range []resumeFixture{
		{video: "libx264", audio: "aac", mode: "remux"},
		{video: "libx264", audio: "eac3", mode: "audio-transcode"},
		{video: "mpeg4", audio: "aac", mode: "transcode"},
		{video: "libx264", audio: "aac", mode: "remux", prepared: true},
		{video: "libx264", audio: "eac3", mode: "audio-transcode", prepared: true},
	} {
		name := fixture.mode
		if fixture.prepared {
			name += "-prepared"
		}
		t.Run(name, func(t *testing.T) {
			assertCopiedResume(t, ffmpeg, ffprobe, fixture)
		})
	}
}

type resumeFixture struct {
	video, audio, mode string
	prepared           bool
}

func assertCopiedResume(t *testing.T, ffmpeg, ffprobe string, fixture resumeFixture) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	media := t.TempDir()
	generate := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=160x90:rate=24:duration=600", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=600", //nolint:gosec // G204: tool path comes from LookPath; arguments are fixed synthetic fixtures.
		"-c:v", fixture.video, "-bf", "3", "-g", "96", "-keyint_min", "96", "-sc_threshold", "0", "-c:a", fixture.audio, "-ac", "2", filepath.Join(media, "Resume Test (2020).mkv"))
	if output, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("generate B-frame fixture: %v: %s", err, output)
	}
	logs := captureCopiedLogs(t, 64<<10)
	defer func() {
		if t.Failed() {
			t.Logf("RESUME_LOGS %s", logs.snapshot())
		}
	}()
	handler, id := formatTestItem(t, server.Config{Lifecycle: ctx, MediaDir: media, CacheDir: t.TempDir(), FFmpeg: ffmpeg, FFprobe: ffprobe})
	var info struct {
		Compatible     string
		CompatiblePlan playback.PlaybackPlan
	}
	mustJSON(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback?videoCodecs=h264&audioCodecs=aac", nil), &info)
	if info.CompatiblePlan.Mode != fixture.mode {
		t.Fatalf("fixture plan = %s; want %s", info.CompatiblePlan.Mode, fixture.mode)
	}
	if fixture.prepared {
		awaitResumePreparation(t, handler, id, info.Compatible)
	}
	master := speedTestGET(t, ctx, handler, info.Compatible)
	base := info.Compatible[:strings.LastIndex(info.Compatible, "/")+1]
	variant := base + speedTestURIs(master)[0]
	playlist := speedTestGET(t, ctx, handler, variant)
	starts, names := resumePlaylistStarts(t, playlist)
	if len(names) < 100 {
		t.Fatalf("resume fixture playlist has %d segments", len(names))
	}
	first := len(names) * 3 / 4
	directory := variant[:strings.LastIndex(variant, "/")+1]
	initialization := speedTestGET(t, ctx, handler, directory+"init.mp4")
	for number := first; number < first+3; number++ {
		fragment := append(bytes.Clone(initialization), speedTestGET(t, ctx, handler, directory+names[number])...)
		assertResumeFragment(t, ctx, ffprobe, fragment, names[number], starts[number])
	}
	if !resumeLogged(logs.snapshot(), "process_started", first) {
		t.Fatalf("no encode started at resume segment %d; the fixture did not exercise a mid-file start", first)
	}
	if !resumeLogged(logs.snapshot(), "media_ready", first) {
		t.Errorf("resume encode at segment %d did not log media_ready", first)
	}
}

func resumePlaylistStarts(t *testing.T, playlist []byte) ([]float64, []string) {
	t.Helper()
	var starts []float64
	var names []string
	elapsed := 0.0
	duration := 0.0
	for _, line := range strings.Split(string(playlist), "\n") {
		if value, found := strings.CutPrefix(line, "#EXTINF:"); found {
			parsed, err := strconv.ParseFloat(strings.TrimSuffix(value, ","), 64)
			if err != nil {
				t.Fatalf("playlist duration %q: %v", line, err)
			}
			duration = parsed
		} else if line != "" && !strings.HasPrefix(line, "#") {
			starts, names = append(starts, elapsed), append(names, line)
			elapsed += duration
		}
	}
	return starts, names
}

func assertResumeFragment(t *testing.T, ctx context.Context, ffprobe string, fragment []byte, name string, start float64) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fragment.mp4")
	if err := os.WriteFile(path, fragment, 0o600); err != nil {
		t.Fatal(err)
	}
	probe := exec.CommandContext(ctx, ffprobe, "-v", "error", "-show_entries", "packet=codec_type,pts_time", "-of", "csv=p=0", path) //nolint:gosec // G204: tool path comes from LookPath; the fragment is a temporary test file.
	output, err := probe.CombinedOutput()
	if err != nil {
		t.Fatalf("%s probe: %v: %s", name, err, output)
	}
	first := map[string]float64{}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		kind, value, _ := strings.Cut(line, ",")
		pts, err := strconv.ParseFloat(value, 64)
		if err != nil {
			continue
		}
		if previous, seen := first[kind]; !seen || pts < previous {
			first[kind] = pts
		}
	}
	for _, kind := range []string{"video", "audio"} {
		pts, seen := first[kind]
		if !seen {
			t.Errorf("%s has no %s packets; playlist start %.3f", name, kind, start)
			continue
		}
		if math.Abs(pts-start) > 0.25 {
			t.Errorf("%s first %s packet at %.3f; playlist start %.3f", name, kind, pts, start)
		}
	}
}

func awaitResumePreparation(t *testing.T, handler http.Handler, id, source string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		response := prepareSource(t, handler, id, source)
		if strings.Contains(response.Body.String(), `"state":"ready"`) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("resume fixture preparation did not become ready: %d %s", response.Code, response.Body.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func resumeLogged(logs, phase string, segment int) bool {
	for _, line := range strings.Split(logs, "\n") {
		if strings.Contains(line, `"phase":"`+phase+`"`) && strings.Contains(line, fmt.Sprintf(`"segment_start":%d,`, segment)) {
			return true
		}
	}
	return false
}
