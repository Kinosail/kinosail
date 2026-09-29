package server_test

import (
	"context"
	"encoding/binary"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/playback"
)

func TestVideoTranscodingAppliesAudioEnhancementsToEveryRendition(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	handler, id, ffmpeg := videoAudioEnhancementFixture(t, ctx)
	host := httptest.NewServer(handler)
	defer host.Close()
	var preferences map[string]any
	mustJSON(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/me/media-preferences", nil), &preferences)
	baseline := make(map[string]float64)
	for _, test := range []struct {
		name            string
		dialogue, night bool
	}{
		{"plain", false, false},
		{"dialogue", true, false},
		{"night", false, true},
		{"both", true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			settings := preferences["playback"].(map[string]any)
			settings["dialogueBoost"], settings["nightMode"] = test.dialogue, test.night
			assertAPIBody(t, apiCall(t, handler, "", http.MethodPut, "/api/v1/me/media-preferences", preferences), http.StatusOK)
			observed := enhancedVideoRenditions(t, ctx, handler, id, host.URL, ffmpeg)
			for line, rms := range observed {
				if test.name == "plain" {
					baseline[line] = rms
				} else if original := baseline[line]; original <= 0 || rms < original*1.3 {
					t.Errorf("%s audio RMS = %.6f; plain = %.6f; selected enhancement did not change the delivered audio", line, rms, original)
				}
				t.Logf("%s audio RMS = %.6f", line, rms)
			}
			if len(observed) == 0 || len(observed) != len(baseline) {
				t.Fatalf("enhancement changed the rendition count: %d, baseline %d", len(observed), len(baseline))
			}
		})
	}
}

func videoAudioEnhancementFixture(t *testing.T, ctx context.Context) (http.Handler, string, string) {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for the video enhancement integration test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required for the video enhancement integration test")
	}
	media := t.TempDir()
	// MPEG-4 video requires conversion; quiet speech-frequency audio exposes
	// ignored enhancement settings through the delivered PCM, not argument text.
	command := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=960x540:rate=24:duration=6", "-f", "lavfi", "-i", "sine=frequency=1600:sample_rate=48000:duration=6", "-af", "volume=0.2", "-c:v", "mpeg4", "-q:v", "8", "-c:a", "pcm_s16le", filepath.Join(media, "Dialog.mkv")) //nolint:gosec // Fixed synthetic media and tool paths from LookPath.
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate video: %v: %s", err, output)
	}
	handler, id := formatTestItem(t, server.Config{Lifecycle: ctx, MediaDir: media, CacheDir: t.TempDir(), FFmpeg: ffmpeg, FFprobe: ffprobe}) //nolint:contextcheck // The Server lifecycle carries ctx through configuration.
	return handler, id, ffmpeg
}

func enhancedVideoRenditions(t *testing.T, ctx context.Context, handler http.Handler, id, origin, ffmpeg string) map[string]float64 {
	t.Helper()
	var result struct {
		Compatible     string
		CompatiblePlan playback.PlaybackPlan
	}
	mustJSON(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback?videoCodecs=h264", nil), &result)
	if result.CompatiblePlan.Mode != "transcode" {
		t.Fatalf("fixture must convert video: %+v", result)
	}
	master := apiCall(t, handler, "", http.MethodGet, result.Compatible, nil)
	assertAPIBody(t, master, http.StatusOK, "#EXT-X-STREAM-INF:")
	base, err := url.Parse(origin + result.Compatible)
	if err != nil {
		t.Fatal(err)
	}
	observed := make(map[string]float64)
	for _, line := range strings.Split(master.Body.String(), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		reference, err := url.Parse(line)
		if err != nil {
			t.Fatal(err)
		}
		observed[line] = enhancedVideoAudioRMS(t, ctx, ffmpeg, base.ResolveReference(reference).String())
	}
	return observed
}

func enhancedVideoAudioRMS(t *testing.T, ctx context.Context, ffmpeg, source string) float64 {
	t.Helper()
	command := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-i", source, "-map", "0:a:0", "-ss", "2", "-t", "2", "-ac", "1", "-ar", "48000", "-f", "s16le", "-") //nolint:gosec // The source is an HLS URL served by this test's local HTTP server.
	var diagnostic strings.Builder
	command.Stderr = &diagnostic
	output, err := command.Output()
	if err != nil || len(output) < 48000*2*2 {
		t.Fatalf("decode enhanced video audio: %d bytes, %v: %s", len(output), err, diagnostic.String())
	}
	var sum float64
	for index := 0; index+1 < len(output); index += 2 {
		sample := float64(binary.LittleEndian.Uint16(output[index:]))
		if sample >= 32768 {
			sample -= 65536
		}
		sum += (sample / 32768) * (sample / 32768)
	}
	return math.Sqrt(sum / float64(len(output)/2))
}
