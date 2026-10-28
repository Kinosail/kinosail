package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestLastSegmentBeforeSkippedEndCreditsAnswersWithoutReadingTheCredits(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for the skipped credits integration test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required for the skipped credits integration test")
	}
	for _, test := range []struct{ encoder, codecs, mode string }{
		{"libx264", "h264", "transcode"},
		{"libx265", "h264", "transcode"},
	} {
		t.Run(test.encoder, func(t *testing.T) {
			assertSkippedTailSeek(t, ffmpeg, ffprobe, test.encoder, test.codecs, test.mode)
		})
	}
}

func assertSkippedTailSeek(t *testing.T, ffmpeg, ffprobe, encoder, codecs, mode string) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	media, tools := t.TempDir(), t.TempDir()
	command := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24:duration=140", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=140", "-c:v", encoder, "-preset", "ultrafast", "-g", "24", "-keyint_min", "24", "-sc_threshold", "0", "-c:a", "aac", "-ac", "2", filepath.Join(media, "Film (2020).mkv"))
	if encoder == "libx265" {
		command.Args = append(command.Args[:len(command.Args)-1], "-x265-params", "log-level=error:scenecut=0", command.Args[len(command.Args)-1])
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate credits fixture: %v: %s", err, output)
	}
	throttled := filepath.Join(tools, "ffmpeg")
	writeExecutable(t, throttled, "#!/bin/sh\ncase \" $* \" in *\" -f hls \"*) exec '"+ffmpeg+"' -readrate 2 \"$@\" ;; esac\nexec '"+ffmpeg+"' \"$@\"\n")
	handler := server.New(server.Config{Lifecycle: ctx, MediaDir: media, CacheDir: t.TempDir(), DataDir: t.TempDir(), FFmpeg: throttled, FFprobe: ffprobe})
	var library struct{ Items []struct{ ID string } }
	copiedSeekJSON(t, handler, "/api/v1/library", &library)
	if len(library.Items) != 1 {
		t.Fatalf("library items = %d", len(library.Items))
	}
	id := library.Items[0].ID
	skippedTailPOST(t, handler, "/markers/"+id, "type=credits&start=40.1&end=140")
	skippedTailPOST(t, handler, "/settings/playback", "mode=compatible&markers=credits")
	var answer struct {
		Compatible     string
		CompatiblePlan struct{ Mode string }
	}
	copiedSeekJSON(t, handler, "/api/v1/items/"+id+"/playback?videoCodecs="+codecs+"&audioCodecs=aac&hdrFormats=sdr&maxAudioChannels=2", &answer)
	if answer.CompatiblePlan.Mode != mode || !strings.Contains(answer.Compatible, "-k") {
		t.Fatalf("fixture plan = %q %q; want %s without the credits", answer.CompatiblePlan.Mode, answer.Compatible, mode)
	}
	master := copiedSeekGET(t, handler, answer.Compatible)
	rendition := answer.Compatible[:strings.LastIndex(answer.Compatible, "/")+1] + strings.TrimSpace(lastPlaylistURI(master))
	_, names := playlistSegmentStarts(copiedSeekGET(t, handler, rendition))
	if len(names) < 10 || len(names) > 25 {
		t.Fatalf("playlist without credits lists %d segments", len(names))
	}
	started := time.Now()
	last := copiedSeekGET(t, handler, rendition[:strings.LastIndex(rendition, "/")+1]+names[len(names)-1])
	if elapsed := time.Since(started); elapsed > 15*time.Second || len(last) == 0 {
		t.Fatalf("last segment before the credits took %v", elapsed)
	}
}

func skippedTailPOST(t *testing.T, handler http.Handler, path, form string) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(form))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("POST %s = %d %q", path, response.Code, response.Body.String())
	}
}
