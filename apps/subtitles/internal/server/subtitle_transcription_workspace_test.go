package server_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestStartupRemovesStaleTranscriptionWorkspacesOnly(t *testing.T) {
	t.Parallel()
	cache, outside := t.TempDir(), t.TempDir()
	stale := filepath.Join(cache, "kinosail-transcription-123456")
	writeTestFile(t, filepath.Join(stale, "audio.wav"), "pcm")
	writeTestFile(t, filepath.Join(stale, "transcript.json"), "{}")
	writeTestFile(t, filepath.Join(outside, "audio.wav"), "keep")
	if err := os.Symlink(outside, filepath.Join(cache, "kinosail-transcription-link")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(cache, "kinosail-transcription-note"), "keep")
	writeTestFile(t, filepath.Join(cache, "subtitle-speech", "kinosail-transcription-nested", "audio.wav"), "keep")
	server.New(server.Config{MediaDir: t.TempDir(), DataDir: t.TempDir(), CacheDir: cache})
	if _, err := os.Lstat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale transcription workspace remains after startup: %v", err)
	}
	for _, kept := range []string{
		filepath.Join(outside, "audio.wav"),
		filepath.Join(cache, "kinosail-transcription-link"),
		filepath.Join(cache, "kinosail-transcription-note"),
		filepath.Join(cache, "subtitle-speech", "kinosail-transcription-nested", "audio.wav"),
	} {
		if _, err := os.Lstat(kept); err != nil {
			t.Fatalf("startup removed unrelated cache entry %s: %v", filepath.Base(kept), err)
		}
	}
}
