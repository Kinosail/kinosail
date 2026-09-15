package mediaprobe

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func waitEmbeddedFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		// Opening the marker precedes writing it; cancellation must wait for the write.
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("subtitle extraction did not reach the expected step")
		case <-ticker.C:
		}
	}
}

func TestEmbeddedSharesExtractionAndCanceledWaitersDoNotStopOwner(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	item := library.Item{ID: "film", Path: filepath.Join(root, "film.mkv")}
	calls, release := filepath.Join(root, "calls"), filepath.Join(root, "release")
	ffmpeg := filepath.Join(root, "ffmpeg")
	writeProbeScript(t, ffmpeg, "#!/bin/sh\nprintf x >> '"+calls+"'\nwhile [ ! -f '"+release+"' ]; do sleep 0.01; done\nprintf '%s' '"+testWebVTT+"'\n")
	probe := embeddedTestProbe(item)
	options := EmbeddedOptions{FFmpeg: ffmpeg, CacheDir: filepath.Join(root, "cache")}
	owner := make(chan error, 1)
	go func() { _, err := probe.Embedded(t.Context(), item, 3, options); owner <- err }()
	waitEmbeddedFile(t, calls)
	waiter, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := probe.Embedded(waiter, item, 3, options); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled waiter = %v", err)
	}
	other := make(chan error, 1)
	go func() { _, err := probe.Embedded(t.Context(), item, 3, options); other <- err }()
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, result := range []<-chan error{owner, other} {
		if err := <-result; err != nil {
			t.Fatal(err)
		}
	}
	if data, err := os.ReadFile(calls); err != nil || string(data) != "x" {
		t.Fatalf("duplicate extraction: %q %v", data, err)
	}
}

func TestEmbeddedViewerRetriesCanceledBackgroundOwner(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	item := library.Item{ID: "film", Path: filepath.Join(root, "film.mkv")}
	calls, release := filepath.Join(root, "calls"), filepath.Join(root, "release")
	ffmpeg := filepath.Join(root, "ffmpeg")
	writeProbeScript(t, ffmpeg, "#!/bin/sh\nprintf x >> '"+calls+"'\nwhile [ ! -f '"+release+"' ]; do sleep 0.01; done\nprintf '%s' '"+testWebVTT+"'\n")
	probe := embeddedTestProbe(item)
	options := EmbeddedOptions{FFmpeg: ffmpeg, CacheDir: filepath.Join(root, "cache")}
	background, cancel := context.WithCancel(t.Context())
	owner := make(chan error, 1)
	go func() { _, err := probe.Embedded(background, item, 3, options); owner <- err }()
	waitEmbeddedFile(t, calls)
	viewer := make(chan error, 1)
	go func() { _, err := probe.Embedded(t.Context(), item, 3, options); viewer <- err }()
	cancel()
	if err := <-owner; !errors.Is(err, context.Canceled) {
		t.Fatalf("background cancellation = %v", err)
	}
	if _, err := os.Stat(embeddedSubtitlePath(options.CacheDir, item, 3)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled extraction published a cache: %v", err)
	}
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := <-viewer; err != nil {
		t.Fatalf("viewer inherited background cancellation: %v", err)
	}
	if data, err := os.ReadFile(calls); err != nil || string(data) != "xx" {
		t.Fatalf("retry extraction calls: %q %v", data, err)
	}
}

func TestEmbeddedDoesNotRunPlaybackEnrichment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	item := library.Item{ID: "film", Path: filepath.Join(root, "film.mkv")}
	ffmpeg := filepath.Join(root, "ffmpeg")
	writeProbeScript(t, ffmpeg, "#!/bin/sh\nprintf '%s' '"+testWebVTT+"'\n")
	probe := embeddedTestProbe(item)
	_, err := probe.Embedded(t.Context(), item, 3, EmbeddedOptions{FFmpeg: ffmpeg, Enrichment: Enrichment{Chapters: func(context.Context, library.Item, float64, []Chapter) []Chapter {
		t.Error("subtitle lookup performed playback enrichment")
		return nil
	}}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEmbeddedRejectsChangingSourceBeforeCachePublication(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	item := library.Item{ID: "film", Path: filepath.Join(root, "film.mkv")}
	if err := os.WriteFile(item.Path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffmpeg := filepath.Join(root, "ffmpeg")
	writeProbeScript(t, ffmpeg, "#!/bin/sh\nprintf changed-content > '"+item.Path+"'\nprintf '%s' '"+testWebVTT+"'\n")
	probe := embeddedTestProbe(item)
	cache := filepath.Join(root, "cache")
	_, err := probe.Embedded(t.Context(), item, 3, EmbeddedOptions{FFmpeg: ffmpeg, CacheDir: cache})
	if err == nil || !strings.Contains(err.Error(), "source changed") {
		t.Fatalf("source replacement = %v", err)
	}
	if _, err := os.Stat(cache); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("changing source published cache: %v", err)
	}
}
