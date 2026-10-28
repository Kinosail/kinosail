package server_test

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestCopiedPlaybackSeeksToListedSegmentsBeforeTheCopyReachesThem(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for the copied seek integration test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required for the copied seek integration test")
	}
	for _, encoder := range []string{"libx264", "libx265"} {
		t.Run(encoder, func(t *testing.T) {
			assertCopiedProjectionSeek(t, ffmpeg, ffprobe, encoder)
		})
	}
}

func assertCopiedProjectionSeek(t *testing.T, ffmpeg, ffprobe, encoder string) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	media, tools := t.TempDir(), t.TempDir()
	command := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24:duration=600", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=600", "-c:v", encoder, "-preset", "ultrafast", "-g", "2000", "-sc_threshold", "0", "-force_key_frames", irregularKeyframeTimes(600), "-c:a", "eac3", "-ac", "6", filepath.Join(media, "Film (2020).mkv"))
	if encoder == "libx265" {
		command.Args = append(command.Args[:len(command.Args)-1], "-x265-params", "log-level=error:scenecut=0", "-tag:v", "hvc1", command.Args[len(command.Args)-1])
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate irregular keyframe fixture: %v: %s", err, output)
	}
	throttled := filepath.Join(tools, "ffmpeg")
	writeExecutable(t, throttled, "#!/bin/sh\ncase \" $* \" in *\" -f hls \"*) exec '"+ffmpeg+"' -readrate 20 \"$@\" ;; esac\nexec '"+ffmpeg+"' \"$@\"\n")
	cache := t.TempDir()
	handler := server.New(server.Config{Lifecycle: ctx, MediaDir: media, CacheDir: cache, FFmpeg: throttled, FFprobe: ffprobe})
	var library struct{ Items []struct{ ID string } }
	copiedSeekJSON(t, handler, "/api/v1/library", &library)
	if len(library.Items) != 1 {
		t.Fatalf("library items = %d", len(library.Items))
	}
	var answer struct {
		Compatible     string
		CompatiblePlan struct{ Mode string }
	}
	copiedSeekJSON(t, handler, "/api/v1/items/"+library.Items[0].ID+"/playback?videoCodecs=h264,hevc&audioCodecs=aac,mp3,ac3,eac3&hdrFormats=sdr&maxAudioChannels=2", &answer)
	if answer.CompatiblePlan.Mode != "audio-transcode" {
		t.Fatalf("fixture plan = %q; want audio-transcode with copied video", answer.CompatiblePlan.Mode)
	}
	master := copiedSeekGET(t, handler, answer.Compatible)
	rendition := answer.Compatible[:strings.LastIndex(answer.Compatible, "/")+1] + strings.TrimSpace(lastPlaylistURI(master))
	playlist := copiedSeekGET(t, handler, rendition)
	starts, names := playlistSegmentStarts(playlist)
	if len(names) < 20 || !strings.Contains(string(playlist), "#EXT-X-ENDLIST") {
		t.Fatalf("copied playlist must list the whole movie: %d segments\n%s", len(names), playlist)
	}
	waitForCopiedSegments(t, cache, 3)
	base := rendition[:strings.LastIndex(rendition, "/")+1]
	initialization := copiedSeekGET(t, handler, base+"init.mp4")
	previous := -1.0
	for _, index := range []int{len(names) / 2, len(names)/2 + 1, len(names) - 1} {
		segment := copiedSeekGET(t, handler, base+names[index])
		first := firstVideoTime(t, ctx, ffprobe, append(append([]byte(nil), initialization...), segment...))
		if math.Abs(first-starts[index]) > 12.5 || first <= previous {
			t.Errorf("%s starts at %.3fs after %.3fs; playlist places it at %.3fs", names[index], first, previous, starts[index])
		}
		previous = first
	}
	if again := copiedSeekGET(t, handler, rendition); string(again) != string(playlist) {
		t.Errorf("a later playlist request renumbered the copied stream:\n%.700s\nfirst:\n%.700s", again, playlist)
	}
}

func waitForCopiedSegments(t *testing.T, cache string, count int) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		playlists, _ := filepath.Glob(filepath.Join(cache, "*", "*", "index.m3u8"))
		for _, playlist := range playlists {
			if data, err := os.ReadFile(playlist); err == nil && strings.Count(string(data), "segment-") >= count && !strings.Contains(string(data), "#EXT-X-ENDLIST") {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("copied stream did not publish %d irregular segments", count)
}

func irregularKeyframeTimes(duration float64) string {
	gaps := []float64{10.427, 8.967, 5.589, 3.2, 11.7, 6.1, 1.71, 5.047, 9.3, 7.7}
	times, at := []string{"0"}, 0.0
	for index := 0; at < duration; index++ {
		at += gaps[index%len(gaps)]
		times = append(times, strconv.FormatFloat(at, 'f', 3, 64))
	}
	return strings.Join(times, ",")
}

func copiedSeekGET(t *testing.T, handler http.Handler, path string) []byte {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	if response.Code != http.StatusOK || response.Body.Len() == 0 {
		t.Fatalf("GET %s = %d", path, response.Code)
	}
	return response.Body.Bytes()
}

func copiedSeekJSON(t *testing.T, handler http.Handler, path string, value any) {
	t.Helper()
	if err := json.Unmarshal(copiedSeekGET(t, handler, path), value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func lastPlaylistURI(manifest []byte) string {
	uri := ""
	for _, line := range strings.Split(string(manifest), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			uri = line
		}
	}
	return uri
}

func playlistSegmentStarts(manifest []byte) ([]float64, []string) {
	var starts []float64
	var names []string
	at, length := 0.0, 0.0
	for _, line := range strings.Split(string(manifest), "\n") {
		line = strings.TrimSpace(line)
		if value, found := strings.CutPrefix(line, "#EXTINF:"); found {
			length, _ = strconv.ParseFloat(strings.TrimSuffix(value, ","), 64)
			continue
		}
		if strings.HasPrefix(line, "segment-") {
			starts, names = append(starts, at), append(names, line)
			at += length
		}
	}
	return starts, names
}

func firstVideoTime(t *testing.T, ctx context.Context, ffprobe string, fragment []byte) float64 {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fragment.mp4")
	if err := os.WriteFile(path, fragment, 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "v:0", "-show_entries", "packet=pts_time", "-of", "csv=p=0", "-read_intervals", "%+#1", path).Output() //nolint:gosec // G204: ffprobe comes from LookPath and the path is a test temporary file.
	value, parseErr := strconv.ParseFloat(strings.TrimSpace(strings.SplitN(string(output), "\n", 2)[0]), 64)
	if err != nil || parseErr != nil {
		t.Fatalf("segment has no readable video: %v %v %q", err, parseErr, output)
	}
	return value
}
