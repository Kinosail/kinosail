package mediaprobe

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestCachedScanFactsUsesScanVersionWithoutReadingMedia(t *testing.T) { //nolint:cyclop,gocognit // Memory and restarted cache readers share the same deleted source fixture.
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
	writeProbeScript(t, executable, "#!/bin/sh\nprintf x >> '"+calls+"'\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\"}],\"format\":{\"format_name\":\"mkv\"}}'\n")
	probe := New(executable)
	probe.ConfigureCache(root)
	if _, found := probe.CachedScanFacts(item); found {
		t.Fatal("cold scan cache reported facts")
	}
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Fatalf("scan lookup ran probe: %v", err)
	}
	probe.Facts(t.Context(), item)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	restarted := New(executable)
	restarted.ConfigureCache(root)
	for _, reader := range []*Probe{probe, restarted} {
		if facts, found := reader.CachedScanFacts(item); !found || facts.Video.Codec != "h264" {
			t.Fatalf("scan facts = %#v, %v", facts, found)
		}
		assertStaleScanRejected(t, item, reader.CachedScanFacts)
		if _, found := reader.CachedFacts(item); found {
			t.Fatal("live lookup accepted a removed source")
		}
	}
	if data, err := os.ReadFile(calls); err != nil || string(data) != "x" {
		t.Fatalf("scan lookups launched FFprobe: %q %v", data, err)
	}
}
