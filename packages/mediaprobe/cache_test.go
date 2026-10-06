package mediaprobe

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestProbeCacheRemovesRestartProbeFromPlaybackPath(t *testing.T) {
	root, cache := t.TempDir(), t.TempDir()
	media, calls, executable := filepath.Join(root, "film.mp4"), filepath.Join(root, "calls"), filepath.Join(root, "ffprobe")
	if err := os.WriteFile(media, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprintf x >> '" + calls + "'\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\"}],\"format\":{\"format_name\":\"mp4\",\"duration\":\"120\"}}'\n"
	writeProbeScript(t, executable, script)
	item := library.Item{ID: "film", Kind: "video", Path: media}
	for range 2 {
		probe := New(executable)
		probe.cacheDir = cache
		if result := probe.Inspect(context.Background(), item, Enrichment{}); result.Video.Codec != "h264" {
			t.Fatalf("probe = %#v", result)
		}
	}
	if data, err := os.ReadFile(calls); err != nil || string(data) != "x" {
		t.Fatalf("FFprobe calls = %q, error = %v", data, err)
	}
}

func TestProbeCoalescesConcurrentRequestsForOneSource(t *testing.T) {
	root := t.TempDir()
	media, calls, executable := filepath.Join(root, "film.mp4"), filepath.Join(root, "calls"), filepath.Join(root, "ffprobe")
	if err := os.WriteFile(media, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprintf x >> '" + calls + "'\nsleep 0.1\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\"}],\"format\":{\"format_name\":\"mp4\"}}'\n"
	writeProbeScript(t, executable, script)
	probe := New(executable)
	item := library.Item{ID: "film", Kind: "video", Path: media}
	var wait sync.WaitGroup
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if result := probe.Inspect(context.Background(), item, Enrichment{}); result.Video.Codec != "h264" {
				t.Errorf("probe = %#v", result)
			}
		}()
	}
	wait.Wait()
	if data, err := os.ReadFile(calls); err != nil || string(data) != "x" {
		t.Fatalf("FFprobe calls = %q, error = %v", data, err)
	}
}

func writeProbeScript(t *testing.T, path, script string) {
	t.Helper()
	unlock := lockProbeFixturePublication()
	defer unlock()
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(temporary, 0o700); err != nil { //nolint:gosec // A test-only executable must be runnable.
		t.Fatal(err)
	}
	if err := os.Rename(temporary, path); err != nil {
		t.Fatal(err)
	}
}
