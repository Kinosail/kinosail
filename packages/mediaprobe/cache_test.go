package mediaprobe

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
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

func TestProbeCacheRejectsStaleMalformedAndOversizedState(t *testing.T) {
	root, cache := t.TempDir(), t.TempDir()
	media := filepath.Join(root, "film.mp4")
	if err := os.WriteFile(media, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	probe := New("unused")
	probe.cacheDir = cache
	item := library.Item{ID: "film", Kind: "video", Path: media}
	path := probe.cachePath(item.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string][]byte{
		"malformed": []byte(`{"schema":1`),
		"unknown":   []byte(`{"schema":1,"version":"` + playback.SourceVersion(media) + `","result":{},"extra":true}`),
		"stale":     []byte(`{"schema":1,"version":"old","result":{}}`),
		"oversized": make([]byte, probeCacheLimit+1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, content, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, found := probe.load(item, playback.SourceVersion(media)); found {
				t.Fatal("invalid cache was accepted")
			}
		})
	}
}
