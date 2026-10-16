package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSpeechAnalysisIsSharedAcrossLanguagesAndRestarts(t *testing.T) {
	t.Parallel()
	directory, cache := t.TempDir(), t.TempDir()
	calls := filepath.Join(directory, "calls")
	ffmpeg := filepath.Join(directory, "ffmpeg")
	writeSubtitleFactsExecutable(t, ffmpeg, "#!/bin/sh\necho run >> '"+calls+"'\ndd if=/dev/zero bs=1920000 count=1 2>/dev/null\n")
	item := library.Item{ID: "0123456789abcdef", Kind: "video", Path: filepath.Join(directory, "Film.mkv")}
	if err := os.WriteFile(item.Path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	analyses := func() int {
		data, _ := os.ReadFile(calls)
		return strings.Count(string(data), "run")
	}
	first := newSubtitleSynchronizer(ffmpeg)
	first.cacheDir = cache
	english, err := first.audioReference(t.Context(), item, "en")
	if err != nil || len(english.Speech) != int(time.Minute/subtitleFrame) {
		t.Fatalf("English analysis = %d frames, %v", len(english.Speech), err)
	}
	if _, err = first.audioReference(t.Context(), item, "fr"); err != nil || analyses() != 1 {
		t.Fatalf("second language repeated the same audio analysis: %d runs, %v", analyses(), err)
	}
	restarted := newSubtitleSynchronizer(ffmpeg)
	restarted.cacheDir = cache
	persisted, err := restarted.audioReference(t.Context(), item, "de")
	if err != nil || analyses() != 1 || len(persisted.Speech) != len(english.Speech) || len(persisted.Waveform) != len(english.Waveform) {
		t.Fatalf("restart repeated or changed the analysis: %d runs, %d frames, %v", analyses(), len(persisted.Speech), err)
	}
	for frame := range english.Speech {
		if persisted.Speech[frame] != english.Speech[frame] {
			t.Fatalf("persisted frame %d = %v, analyzed %v", frame, persisted.Speech[frame], english.Speech[frame])
		}
	}
	if err := os.WriteFile(item.Path, []byte("replacement video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.audioReference(t.Context(), item, "en"); err != nil || analyses() != 2 {
		t.Fatalf("replaced media reused stale analysis: %d runs, %v", analyses(), err)
	}
}
