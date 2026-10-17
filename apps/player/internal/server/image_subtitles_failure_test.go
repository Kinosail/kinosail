package server_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestImageSubtitleConversionFailuresKeepBurnInWithoutSideEffects(t *testing.T) {
	ffmpeg, ffprobe := realImageSubtitleTools(t)
	for _, scenario := range []string{"recognition unavailable", "nothing readable", "text track exists"} {
		t.Run(scenario, func(t *testing.T) {
			media, tools, cache := t.TempDir(), t.TempDir(), t.TempDir()
			writeImageSubtitleFixture(t, ffmpeg, filepath.Join(media, "Film.mkv"))
			calls, tesseract := filepath.Join(tools, "calls"), filepath.Join(tools, "tesseract")
			script := strings.Replace(fakeTesseract, "'%s'", "'"+calls+"'", 1)
			switch scenario {
			case "recognition unavailable":
				tesseract = filepath.Join(tools, "missing-tesseract")
			case "nothing readable":
				script = strings.Replace(script, `printf 'Line %d says <hi> & bye\n' "$count"`, `printf ' -- \n'`, 1)
			case "text track exists":
				if err := os.WriteFile(filepath.Join(media, "Film.en.srt"), []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(tools, "tesseract"), []byte(script), 0o700); err != nil { //nolint:gosec // The test double must be executable.
				t.Fatal(err)
			}
			handler, id := formatTestItem(t, server.Config{Lifecycle: t.Context(), MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe, Tesseract: tesseract})
			if scenario == "nothing readable" {
				waitForConvertedImageSubtitle(t, cache, "1.vtt.unreadable")
			} else {
				time.Sleep(1500 * time.Millisecond)
			}
			if converted, _ := filepath.Glob(filepath.Join(cache, "embedded-subtitles", "*", "*", "*.vtt")); len(converted) != 0 {
				t.Fatalf("unexpected converted subtitles: %v", converted)
			}
			if leftovers, _ := filepath.Glob(filepath.Join(cache, "embedded-subtitles", "*", "*", ".ocr-*")); len(leftovers) != 0 {
				t.Fatalf("OCR workspaces were left behind: %v", leftovers)
			}
			used, _ := os.ReadFile(calls)
			if want := map[string]int{"recognition unavailable": 0, "nothing readable": 1, "text track exists": 0}[scenario]; strings.Count(string(used), "\n") != want {
				t.Fatalf("tesseract calls = %q, want %d", used, want)
			}
			if page := apiCall(t, handler, "", "GET", "/watch/"+id+"?subtitle=0", nil); !strings.Contains(page.Body.String(), "The selected subtitles must be added to the video.") {
				t.Fatalf("image subtitle should still burn in: %d", page.Code)
			}
		})
	}
}
