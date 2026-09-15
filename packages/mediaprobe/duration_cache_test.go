package mediaprobe

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func TestDurationCoalescesAndPersistsSourceFacts(t *testing.T) { //nolint:cyclop // Concurrency, persistence, and restart assertions share one probe counter.
	root := t.TempDir()
	media, calls, executable := filepath.Join(root, "film.mp4"), filepath.Join(root, "calls"), filepath.Join(root, "ffprobe")
	if err := os.WriteFile(media, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeProbeScript(t, executable, "#!/bin/sh\nprintf x >> '"+calls+"'\nsleep 0.1\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\"}],\"format\":{\"duration\":\"73\"}}'\n")
	item := library.Item{ID: "film", Kind: "video", Path: media}
	probe := New(executable)
	probe.ConfigureCache(root)
	var wait sync.WaitGroup
	for range 8 {
		wait.Go(func() {
			if got := probe.Duration(t.Context(), item); got != 73 {
				t.Errorf("duration = %v", got)
			}
		})
	}
	wait.Wait()
	restarted := New(executable)
	restarted.ConfigureCache(root)
	for _, reader := range []*Probe{probe, restarted} {
		if reader.Duration(t.Context(), item) != 73 || reader.Facts(t.Context(), item).Video.Codec != "h264" {
			t.Fatal("duration did not populate the shared facts cache")
		}
	}
	if data, err := os.ReadFile(calls); err != nil || string(data) != "x" {
		t.Fatalf("probe executions = %q, %v", data, err)
	}
	if err := os.WriteFile(media, []byte("replacement media"), 0o600); err != nil {
		t.Fatal(err)
	}
	if probe.Duration(t.Context(), item) != 73 {
		t.Fatal("replacement was not probed")
	}
	if data, err := os.ReadFile(calls); err != nil || string(data) != "xx" {
		t.Fatalf("replacement probe executions = %q, %v", data, err)
	}
}

func TestCanceledDurationDoesNotCacheFailure(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "ffprobe")
	writeProbeScript(t, executable, "#!/bin/sh\nprintf '%s' '{\"format\":{\"duration\":\"73\"}}'\n")
	probe := New(executable)
	item := library.Item{ID: "film", Path: "missing"}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if probe.Duration(ctx, item) != 0 {
		t.Fatal("canceled request returned a duration")
	}
	if probe.Duration(t.Context(), item) != 73 {
		t.Fatal("cancellation poisoned the cache")
	}
}

func TestRejectedProbeOutputDoesNotPoisonCache(t *testing.T) {
	for name, output := range map[string]string{
		"malformed":          `{"streams":`,
		"invalid geometry":   `{"streams":[{"codec_type":"video","codec_name":"h264","width":-1}]}`,
		"nonfinite duration": `{"format":{"duration":"NaN"}}`,
		"too many streams":   `{"streams":[` + strings.Repeat(`{},`, 256) + `{ }]}`,
		"too many chapters":  `{"chapters":[` + strings.Repeat(`{},`, 4096) + `{ }]}`,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			media, executable := filepath.Join(root, "film.mp4"), filepath.Join(root, "ffprobe")
			if err := os.WriteFile(media, []byte("media"), 0o600); err != nil {
				t.Fatal(err)
			}
			writeProbeScript(t, executable, "#!/bin/sh\nprintf '%s' '"+output+"'\n")
			probe := New(executable)
			probe.ConfigureCache(root)
			item := library.Item{ID: "film", Path: media}
			probe.Facts(t.Context(), item)
			if _, found := probe.CachedFacts(item); found {
				t.Fatal("rejected probe result was cached")
			}
			if _, err := os.Stat(probe.cachePath(item.ID)); !os.IsNotExist(err) {
				t.Fatalf("rejected result reached disk: %v", err)
			}
			writeProbeScript(t, executable, "#!/bin/sh\nprintf '%s' '{\"format\":{\"duration\":\"73\"}}'\n")
			if probe.Duration(t.Context(), item) != 73 {
				t.Fatal("valid retry did not recover")
			}
		})
	}
}

func TestOldProbeCacheCannotRetainRejectedFacts(t *testing.T) {
	root := t.TempDir()
	media, executable := filepath.Join(root, "film.mp4"), filepath.Join(root, "ffprobe")
	if err := os.WriteFile(media, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeProbeScript(t, executable, "#!/bin/sh\nprintf '%s' '{\"format\":{\"duration\":\"73\"}}'\n")
	probe := New(executable)
	probe.ConfigureCache(root)
	item := library.Item{ID: "film", Path: media}
	path := probe.cachePath(item.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(probeCacheEntry{Schema: 2, Version: playback.SourceVersion(media), Result: Result{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if probe.Duration(t.Context(), item) != 73 {
		t.Fatal("old empty facts prevented a new probe")
	}
}
