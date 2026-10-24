package playback

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEpisodeStillSkipsDarkFramesForTheNextTimestamp(t *testing.T) {
	directory := t.TempDir()
	dark, bright := filepath.Join(directory, "dark.jpg"), filepath.Join(directory, "bright.jpg")
	writeStillFixture(t, dark, func(int, int) color.Color { return color.Gray{Y: 4} })
	writeStillFixture(t, bright, func(x, y int) color.Color { return color.Gray{Y: uint8(40 + (x+y)%160)} })
	calls := filepath.Join(directory, "calls")
	ffmpeg := filepath.Join(directory, "ffmpeg")
	script := "#!/bin/sh\nfor last; do :; done\necho \"$6\" >> " + calls + "\nif [ \"$6\" = 60 ]; then cp " + bright + " \"$last\"; else cp " + dark + " \"$last\"; fi\n"
	writeFakeFFmpeg(t, ffmpeg, script)
	target := filepath.Join(directory, "stills", "episode.jpg")
	if err := GenerateEpisodeStill(t.Context(), directory, ffmpeg, filepath.Join(directory, "source.mkv"), target, 30, 60, 90); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(calls)
	if err != nil || strings.Count(string(data), "\n") != 2 {
		t.Fatalf("frame attempts = %q, %v", data, err)
	}
	if !visibleStill(target) {
		t.Fatal("the dark frame was kept")
	}
}

func TestEpisodeStillKeepsTheLastFrameWhenEveryTimestampIsDark(t *testing.T) {
	directory := t.TempDir()
	dark := filepath.Join(directory, "dark.jpg")
	writeStillFixture(t, dark, func(int, int) color.Color { return color.Gray{Y: 4} })
	ffmpeg := filepath.Join(directory, "ffmpeg")
	writeFakeFFmpeg(t, ffmpeg, "#!/bin/sh\nfor last; do :; done\ncp "+dark+" \"$last\"\n")
	target := filepath.Join(directory, "stills", "episode.jpg")
	if err := GenerateEpisodeStill(t.Context(), directory, ffmpeg, filepath.Join(directory, "source.mkv"), target, 30, 60); err != nil {
		t.Fatal(err)
	}
	if nonEmptyFrame(target) != nil {
		t.Fatal("no fallback frame was kept")
	}
}

func writeStillFixture(t *testing.T, path string, pixel func(int, int) color.Color) {
	t.Helper()
	frame := image.NewRGBA(image.Rect(0, 0, 96, 54))
	for y := range 54 {
		for x := range 96 {
			frame.Set(x, y, pixel(x, y))
		}
	}
	var data bytes.Buffer
	if err := jpeg.Encode(&data, frame, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeFakeFFmpeg(t *testing.T, path, script string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil { //nolint:gosec // The fake ffmpeg must be executable by the test process.
		t.Fatal(err)
	}
}
