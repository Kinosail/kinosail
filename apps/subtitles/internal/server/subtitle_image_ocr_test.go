package server_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

const fakeSubtitleTesseract = `#!/bin/sh
if [ "$1" = "--list-langs" ]; then printf 'List of available languages in "/fake" (2):\neng\nosd\n'; exit 0; fi
printf '%s\n' "$*" >> '%s'
count=0
while IFS= read -r page; do
	[ -s "$page" ] || exit 3
	[ "$count" -gt 0 ] && printf '\f'
	count=$((count + 1))
	printf 'Line %d is read\n' "$count"
done < "$1"
`

func TestSubtitleAppAutomaticallyConvertsImageSubtitlesWithoutProviders(t *testing.T) {
	t.Parallel()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for the image subtitle integration test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required for the image subtitle integration test")
	}
	lifecycle, cancel := context.WithCancel(context.Background())
	defer cancel()
	media, tools, data := t.TempDir(), t.TempDir(), t.TempDir()
	film := filepath.Join(media, "Arrival.mkv")
	arguments := []string{"-v", "error", "-f", "lavfi", "-i", "testsrc2=size=1280x720:rate=12:duration=12", "-i", filepath.Join("testdata", "image-subtitles-eng.sup"), "-map", "0", "-map", "1", "-c:v", "libx264", "-preset", "ultrafast", "-c:s", "copy", "-copyts", "-metadata:s:s:0", "language=eng", film}
	if output, buildErr := exec.CommandContext(t.Context(), ffmpeg, arguments...).CombinedOutput(); buildErr != nil {
		t.Fatalf("generate image subtitle fixture: %v: %s", buildErr, output)
	}
	calls, tesseract := filepath.Join(tools, "calls"), filepath.Join(tools, "tesseract")
	writeExecutable(t, tesseract, strings.Replace(fakeSubtitleTesseract, "'%s'", "'"+calls+"'", 1))
	snapshot, err := configuration.Load(data, "", func(key string) (string, bool) {
		if key == "KINOSAIL_TESSERACT" {
			return tesseract, true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = server.New(server.Config{Lifecycle: lifecycle, SubtitleApp: true, MediaDir: media, DataDir: data, CacheDir: t.TempDir(), FFprobe: ffprobe, FFmpeg: ffmpeg, BackupDir: t.TempDir(), BackupKey: "test-encryption-key", Configuration: snapshot})
	target := filepath.Join(media, "Arrival.en.srt")
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		sidecar, readErr := os.ReadFile(target)
		if readErr == nil {
			text := strings.ReplaceAll(string(sidecar), "\r\n", "\n")
			used, _ := os.ReadFile(calls)
			if !strings.Contains(text, "00:00:01,500 --> 00:00:03,250\nLine 1 is read") || !strings.Contains(text, "00:00:07,000 --> 00:00:09,") || strings.Contains(text, "WEBVTT") || !strings.Contains(string(used), "-l eng") {
				t.Fatalf("sidecar = %q, tesseract calls = %q", text, used)
			}
			cancel()
			time.Sleep(50 * time.Millisecond)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("automatic image subtitle conversion did not write a sidecar")
}
