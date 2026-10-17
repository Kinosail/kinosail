package server_test

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestRealEmbeddedSubtitlesShareOneExtractionPassWithPerStreamBytes(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for the embedded subtitle integration test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required for the embedded subtitle integration test")
	}
	media, tools := t.TempDir(), t.TempDir()
	film := filepath.Join(media, "Film.mkv")
	writeEmbeddedSubtitleFixture(t, ffmpeg, tools, film)
	calls, counting := filepath.Join(tools, "calls"), filepath.Join(tools, "counting-ffmpeg")
	if err = os.WriteFile(counting, []byte("#!/bin/sh\nprintf x >> '"+calls+"'\nexec '"+ffmpeg+"' \"$@\"\n"), 0o700); err != nil { //nolint:gosec // The test wrapper must be executable.
		t.Fatal(err)
	}
	handler, id := formatTestItem(t, server.Config{Lifecycle: t.Context(), MediaDir: media, CacheDir: t.TempDir(), FFmpeg: counting, FFprobe: ffprobe})
	playback := apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback", nil)
	sources := regexp.MustCompile(`/subtitle/`+regexp.QuoteMeta(id)+`/embedded/(\d+)`).FindAllStringSubmatch(playback.Body.String(), -1)
	streams := map[string]bool{}
	for _, source := range sources {
		streams[source[1]] = true
	}
	if playback.Code != http.StatusOK || len(streams) != 3 {
		t.Fatalf("embedded tracks = %d %v %s", playback.Code, streams, playback.Body.String())
	}
	for _, stream := range []string{"4", "2", "3"} {
		reference, referenceErr := exec.CommandContext(t.Context(), ffmpeg, "-v", "error", "-i", film, "-map", "0:"+stream, "-f", "webvtt", "pipe:1").Output()
		if referenceErr != nil || !streams[stream] {
			t.Fatalf("reference stream %s: %v %v", stream, referenceErr, streams)
		}
		response := apiCall(t, handler, "", http.MethodGet, "/subtitle/"+id+"/embedded/"+stream, nil)
		if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), reference) {
			t.Fatalf("stream %s = %d\n%q\nwant\n%q", stream, response.Code, response.Body.String(), reference)
		}
	}
	if data, readErr := os.ReadFile(calls); readErr != nil || string(data) != "x" {
		t.Fatalf("FFmpeg passes = %q, %v", data, readErr)
	}
}

func writeEmbeddedSubtitleFixture(t *testing.T, ffmpeg, tools, film string) {
	t.Helper()
	arguments := []string{"-v", "error", "-f", "lavfi", "-i", "testsrc2=size=160x90:rate=12:duration=30", "-f", "lavfi", "-i", "sine=duration=30"}
	languages := []string{"eng", "fra", "deu"}
	for _, language := range languages {
		var cues strings.Builder
		for cue := range 14 {
			fmt.Fprintf(&cues, "%d\n00:00:%02d,000 --> 00:00:%02d,750\n%s line %d & <b>more</b>\n\n", cue+1, cue*2, cue*2+1, language, cue)
		}
		path := filepath.Join(tools, language+".srt")
		if err := os.WriteFile(path, []byte(cues.String()), 0o600); err != nil {
			t.Fatal(err)
		}
		arguments = append(arguments, "-i", path)
	}
	arguments = append(arguments, "-map", "0", "-map", "1", "-map", "2", "-map", "3", "-map", "4", "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-c:s:0", "srt", "-c:s:1", "ass", "-c:s:2", "srt")
	for position, language := range languages {
		arguments = append(arguments, fmt.Sprintf("-metadata:s:s:%d", position), "language="+language)
	}
	if output, err := exec.CommandContext(t.Context(), ffmpeg, append(arguments, film)...).CombinedOutput(); err != nil { //nolint:gosec // The tool path comes from LookPath and the arguments are fixed synthetic fixtures.
		t.Fatalf("generate embedded subtitle fixture: %v: %s", err, output)
	}
}
