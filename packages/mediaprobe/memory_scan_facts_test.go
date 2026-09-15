package mediaprobe

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestMemoryScanFactsDoesNotHydrateFromDiskOrRunProbe(t *testing.T) { //nolint:cyclop // Warm and cold reads must preserve the same probe and filesystem evidence.
	t.Parallel()
	root := t.TempDir()
	path, calls := filepath.Join(root, "film.mkv"), filepath.Join(root, "calls")
	if err := os.WriteFile(path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	item := library.Item{ID: "film", Path: path, Kind: "video", Size: info.Size(), Added: info.ModTime()}
	executable := filepath.Join(root, "ffprobe")
	writeProbeScript(t, executable, "#!/bin/sh\nprintf x >> '"+calls+"'\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\"}]}'\n")
	warm := New(executable)
	warm.ConfigureCache(root)
	warm.Facts(t.Context(), item)
	cold := New(executable)
	cold.ConfigureCache(root)
	if _, found := cold.MemoryScanFacts(item); found {
		t.Fatal("memory-only lookup hydrated the persistent cache")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if facts, found := warm.MemoryScanFacts(item); !found || facts.Video.Codec != "h264" {
		t.Fatal("memory lookup touched removed media")
	}
	// The explicit background cache reader can still hydrate a fresh process.
	if _, found := cold.CachedScanFacts(item); !found {
		t.Fatal("persistent scan facts were lost")
	}
	if _, found := cold.MemoryScanFacts(item); !found {
		t.Fatal("hydrated memory facts are unavailable")
	}
	assertStaleScanRejected(t, item, warm.MemoryScanFacts)
	if data, err := os.ReadFile(calls); err != nil || string(data) != "x" {
		t.Fatalf("memory reads launched a probe: %q %v", data, err)
	}
}
