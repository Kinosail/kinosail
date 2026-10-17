package server_test

import (
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/playback"
)

const fakeTesseract = `#!/bin/sh
if [ "$1" = "--list-langs" ]; then printf 'List of available languages in "/fake" (2):\neng\nosd\n'; exit 0; fi
printf '%s\n' "$*" >> '%s'
count=0
while IFS= read -r page; do
	[ -s "$page" ] || exit 3
	[ "$count" -gt 0 ] && printf '\f'
	count=$((count + 1))
	printf 'Line %d says <hi> & bye\n' "$count"
done < "$1"
`

func TestRealImageSubtitlesBecomeTextTracksWithoutBurnIn(t *testing.T) {
	ffmpeg, ffprobe := realImageSubtitleTools(t)
	media, tools, cache := t.TempDir(), t.TempDir(), t.TempDir()
	writeImageSubtitleFixture(t, ffmpeg, filepath.Join(media, "Film.mkv"))
	calls, tesseract := filepath.Join(tools, "calls"), filepath.Join(tools, "tesseract")
	if err := os.WriteFile(tesseract, []byte(strings.Replace(fakeTesseract, "'%s'", "'"+calls+"'", 1)), 0o700); err != nil { //nolint:gosec // The test double must be executable.
		t.Fatal(err)
	}
	handler, id := formatTestItem(t, server.Config{Lifecycle: t.Context(), MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe, Tesseract: tesseract})
	converted := waitForConvertedImageSubtitle(t, cache, "1.vtt")
	data, err := os.ReadFile(converted)
	want := "WEBVTT\n\n00:00:01.500 --> 00:00:03.250\nLine 1 says &lt;hi&gt; &amp; bye\n\n00:00:04.500 --> 00:00:06.000\nLine 2 says &lt;hi&gt; &amp; bye\n\n00:00:07.000 --> 00:00:09.500\nLine 3 says &lt;hi&gt; &amp; bye\n"
	if err != nil || !sameWebVTTWithin(string(data), want, 2) {
		t.Fatalf("converted = %q, %v\nwant %q", data, err, want)
	}
	if used, readErr := os.ReadFile(calls); readErr != nil || strings.Count(string(used), "\n") != 1 || !strings.Contains(string(used), "-l eng") {
		t.Fatalf("tesseract calls = %q, %v", used, readErr)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(cache, "embedded-subtitles", "*", "*", ".ocr-*")); len(leftovers) != 0 {
		t.Fatalf("OCR workspaces were left behind: %v", leftovers)
	}
	playback := apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback", nil)
	if body := playback.Body.String(); playback.Code != http.StatusOK || !strings.Contains(body, "/subtitle/"+id+"/embedded/1") || strings.Contains(body, "/subtitle/"+id+"/embedded/2") {
		t.Fatalf("playback tracks = %d %s", playback.Code, body)
	}
	delivered := apiCall(t, handler, "", http.MethodGet, "/subtitle/"+id+"/embedded/1", nil)
	if delivered.Code != http.StatusOK || delivered.Body.String() != string(data) || !strings.HasPrefix(delivered.Header().Get("Content-Type"), "text/vtt") {
		t.Fatalf("delivered = %d %q", delivered.Code, delivered.Body.String())
	}
	if page := apiCall(t, handler, "", http.MethodGet, "/watch/"+id+"?subtitle=0", nil); page.Code != http.StatusOK || strings.Contains(page.Body.String(), "The selected subtitles must be added to the video.") {
		t.Fatalf("converted subtitle still needs burn-in: %d", page.Code)
	}
	if page := apiCall(t, handler, "", http.MethodGet, "/watch/"+id+"?subtitle=1", nil); page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "The selected subtitles must be added to the video.") {
		t.Fatalf("unconverted image subtitle should still burn in: %d", page.Code)
	}
}

func TestRealImageSubtitleOCRReadsTheBitmapText(t *testing.T) {
	ffmpeg, ffprobe := realImageSubtitleTools(t)
	tesseract, err := exec.LookPath("tesseract")
	if err != nil {
		t.Skip("Tesseract is required for the OCR integration test")
	}
	if languages, listErr := exec.CommandContext(t.Context(), tesseract, "--list-langs").CombinedOutput(); listErr != nil || !strings.Contains(string(languages), "\neng") {
		t.Skip("Tesseract English data is required for the OCR integration test")
	}
	media, cache := t.TempDir(), t.TempDir()
	writeImageSubtitleFixture(t, ffmpeg, filepath.Join(media, "Film.mkv"))
	formatTestItem(t, server.Config{Lifecycle: t.Context(), MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe, Tesseract: tesseract})
	data, err := os.ReadFile(waitForConvertedImageSubtitle(t, cache, "1.vtt"))
	want := "WEBVTT\n\n00:00:01.500 --> 00:00:03.250\nHello there, General.\nSecond row is fine.\n\n00:00:04.500 --> 00:00:06.000\nThe quick brown fox\n\n00:00:07.000 --> 00:00:09.500\njumps over the lazy dog!\n"
	if err != nil || !sameWebVTTWithin(string(data), want, 2) {
		t.Fatalf("OCR = %q, %v\nwant %q", data, err, want)
	}
}

func sameWebVTTWithin(got, want string, milliseconds float64) bool {
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(want, "\n")
	if len(gotLines) != len(wantLines) {
		return false
	}
	for index, line := range wantLines {
		start, end, timed := strings.Cut(line, " --> ")
		gotStart, gotEnd, gotTimed := strings.Cut(gotLines[index], " --> ")
		if !timed {
			if gotLines[index] != line {
				return false
			}
			continue
		}
		if !gotTimed || !nearVTTTime(gotStart, start, milliseconds) || !nearVTTTime(gotEnd, end, milliseconds) {
			return false
		}
	}
	return true
}

func nearVTTTime(got, want string, milliseconds float64) bool {
	gotSeconds, gotErr := playback.ParseVTTTime(got)
	wantSeconds, wantErr := playback.ParseVTTTime(want)
	return gotErr == nil && wantErr == nil && math.Abs(gotSeconds-wantSeconds)*1000 <= milliseconds
}

func realImageSubtitleTools(t *testing.T) (string, string) {
	t.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for the image subtitle integration test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required for the image subtitle integration test")
	}
	return ffmpeg, ffprobe
}

func writeImageSubtitleFixture(t *testing.T, ffmpeg, film string) {
	t.Helper()
	arguments := []string{"-v", "error", "-f", "lavfi", "-i", "testsrc2=size=1280x720:rate=12:duration=12", "-i", filepath.Join("testdata", "image-subtitles-eng.sup"), "-i", filepath.Join("testdata", "image-subtitles-deu.sup"), "-map", "0", "-map", "1", "-map", "2", "-c:v", "libx264", "-preset", "ultrafast", "-c:s", "copy", "-copyts", "-metadata:s:s:0", "language=eng", "-metadata:s:s:1", "language=deu", film}
	if output, err := exec.CommandContext(t.Context(), ffmpeg, arguments...).CombinedOutput(); err != nil {
		t.Fatalf("generate image subtitle fixture: %v: %s", err, output)
	}
}

func waitForConvertedImageSubtitle(t *testing.T, cache, name string) string {
	t.Helper()
	for deadline := time.Now().Add(60 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if found, _ := filepath.Glob(filepath.Join(cache, "embedded-subtitles", "*", "*", name)); len(found) == 1 {
			return found[0]
		}
		if time.Now().After(deadline) {
			t.Fatal("background preparation did not convert the image subtitle")
		}
	}
}
