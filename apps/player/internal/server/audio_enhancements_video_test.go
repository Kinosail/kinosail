package server_test

import (
	"context"
	"encoding/binary"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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
	handler, id, ffmpeg, commands := videoAudioEnhancementFixture(t, ctx)
	host := httptest.NewServer(handler)
	defer host.Close()
	var preferences map[string]any
	mustJSON(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/me/media-preferences", nil), &preferences)
	baseline := make(map[string]float64)
	for _, test := range []struct {
		name            string
		dialogue, night bool
		filters         []string
	}{
		{"plain", false, false, nil},
		{"dialogue", true, false, []string{"equalizer=f=1600"}},
		{"night", false, true, []string{"dynaudnorm="}},
		{"both", true, true, []string{"equalizer=f=1600", "dynaudnorm="}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(commands, nil, 0o600); err != nil {
				t.Fatal(err)
			}
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
			assertEnhancementFilteredOnce(t, commands, test.filters)
		})
	}
}

func videoAudioEnhancementFixture(t *testing.T, ctx context.Context) (http.Handler, string, string, string) {
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
	command := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=960x540:rate=24:duration=6", "-f", "lavfi", "-i", "sine=frequency=1600:sample_rate=48000:duration=6", "-af", "volume=0.2", "-c:v", "mpeg4", "-q:v", "8", "-c:a", "pcm_s16le", filepath.Join(media, "Dialog.mkv"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate video: %v: %s", err, output)
	}
	tools := t.TempDir()
	commands, recorder := filepath.Join(tools, "commands"), filepath.Join(tools, "ffmpeg")
	writeExecutable(t, recorder, "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '"+commands+"'\nexec '"+ffmpeg+"' \"$@\"\n")
	handler, id := formatTestItem(t, server.Config{Lifecycle: ctx, MediaDir: media, CacheDir: t.TempDir(), FFmpeg: recorder, FFprobe: ffprobe}) //nolint:contextcheck // The Server lifecycle carries ctx through configuration.
	return handler, id, ffmpeg, commands
}

func assertEnhancementFilteredOnce(t *testing.T, commands string, filters []string) {
	t.Helper()
	recorded, err := os.ReadFile(commands)
	if err != nil {
		t.Fatal(err)
	}
	presentations := 0
	for _, command := range strings.Split(string(recorded), "\n") {
		if !strings.Contains(command, "-f hls") || len(filters) == 0 {
			continue
		}
		presentations++
		for _, filter := range filters {
			if count := strings.Count(command, filter); count != 1 {
				t.Errorf("presentation applies %q %d times for one ladder: %s", filter, count, command)
			}
		}
	}
	if len(filters) > 0 && presentations == 0 {
		t.Fatalf("no presentation command was recorded: %q", recorded)
	}
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
	command := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-i", source, "-map", "0:a:0", "-ss", "2", "-t", "2", "-ac", "1", "-ar", "48000", "-f", "s16le", "-")
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
