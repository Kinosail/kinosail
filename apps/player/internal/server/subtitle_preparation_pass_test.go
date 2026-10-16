package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubtitlePreparationRevisitsOnlyChangedTitles(t *testing.T) {
	root := t.TempDir()
	calls := filepath.Join(root, "calls")
	ffprobe, ffmpeg := filepath.Join(root, "ffprobe"), filepath.Join(root, "ffmpeg")
	for path, content := range map[string]string{
		ffprobe: "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"index\":2,\"codec_type\":\"subtitle\",\"codec_name\":\"subrip\",\"tags\":{\"language\":\"eng\"}}],\"format\":{\"duration\":\"30\"}}'\n",
		ffmpeg:  "#!/bin/sh\nprintf x >> '" + calls + "'\nprintf 'WEBVTT\\n\\n00:00.000 --> 00:02.000\\nCaptions\\n'\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o700); err != nil { //nolint:gosec // Test executables must be runnable.
			t.Fatal(err)
		}
	}
	items := make([]library.Item, 3, 4)
	for index := range items {
		path := filepath.Join(root, string(rune('a'+index))+".mkv")
		if err := os.WriteFile(path, []byte("media"), 0o600); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		items[index] = library.Item{ID: string(rune('a' + index)), Kind: "video", Path: path, Size: info.Size(), Added: info.ModTime()}
	}
	items = append(items, library.Item{ID: "song", Kind: "audio", Path: filepath.Join(root, "song.flac")})
	acquired := atomic.Int32{}
	newPreparation := func() *subtitlePreparation {
		probe := newMediaProbe(ffprobe)
		probe.ffmpeg, probe.cacheDir = ffmpeg, filepath.Join(root, "cache")
		probe.core.ConfigureCache(probe.cacheDir)
		return &subtitlePreparation{
			probe: probe, language: func() string { return "en" }, prepared: make(map[string]string),
			acquire:  func(context.Context) (func(), error) { acquired.Add(1); return func() {}, nil },
			waitIdle: func(context.Context) error { return nil }, busy: func() bool { return false }, started: func() <-chan struct{} { return nil },
		}
	}
	preparation := newPreparation()
	preparation.prepare(t.Context(), items)
	if acquired.Load() != 3 {
		t.Fatalf("first pass acquired %d background slots, want 3", acquired.Load())
	}
	preparation.prepare(t.Context(), items)
	newPreparation().prepare(t.Context(), items)
	if acquired.Load() != 3 {
		t.Fatalf("prepared titles acquired %d more background slots", acquired.Load()-3)
	}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(items[1].Path, []byte("replaced media"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(items[1].Path)
	if err != nil {
		t.Fatal(err)
	}
	items[1].Size, items[1].Added = info.Size(), info.ModTime()
	preparation.prepare(t.Context(), items)
	data, _ := os.ReadFile(calls)
	if acquired.Load() != 4 || strings.Count(string(data), "x") != 4 {
		t.Fatalf("changed title: slots=%d extractions=%q", acquired.Load(), data)
	}
}
