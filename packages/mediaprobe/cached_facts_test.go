package mediaprobe

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestCachedFactsNeverLaunchesProbeAndRejectsChangedSources(t *testing.T) { //nolint:cyclop // Cache hits and changed-source misses share one counted probe fixture.
	t.Parallel()
	root := t.TempDir()
	media, calls := filepath.Join(root, "film.mkv"), filepath.Join(root, "calls")
	if err := os.WriteFile(media, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(root, "ffprobe")
	writeProbeScript(t, executable, "#!/bin/sh\nprintf x >> '"+calls+"'\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\"}],\"format\":{\"format_name\":\"mkv\"}}'\n")
	item := library.Item{ID: "film", Path: media, Kind: "video"}
	probe := New(executable)
	probe.ConfigureCache(root)
	if _, found := probe.CachedFacts(item); found {
		t.Fatal("cold cache reported known facts")
	}
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Fatalf("cache read started FFprobe: %v", err)
	}
	probe.Facts(t.Context(), item)
	restarted := New(executable)
	restarted.ConfigureCache(root)
	for _, reader := range []*Probe{probe, restarted} {
		if facts, found := reader.CachedFacts(item); !found || facts.Video.Codec != "h264" {
			t.Fatalf("warm cache = %#v, %v", facts, found)
		}
	}
	if err := os.WriteFile(media, []byte("replacement with a different size"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, reader := range []*Probe{probe, restarted} {
		if _, found := reader.CachedFacts(item); found {
			t.Fatal("accepted stale media facts")
		}
	}
	if err := os.Remove(media); err != nil {
		t.Fatal(err)
	}
	if _, found := restarted.CachedFacts(item); found {
		t.Fatal("accepted facts for missing media")
	}
	if data, err := os.ReadFile(calls); err != nil || string(data) != "x" {
		t.Fatalf("cache-only calls ran FFprobe: %q, %v", data, err)
	}
}
