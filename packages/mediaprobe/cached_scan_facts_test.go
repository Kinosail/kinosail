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
	if facts := probe.Facts(t.Context(), item); facts.Video.Codec != "h264" {
		t.Fatalf("initial probe facts = %#v", facts)
	}
	if data, err := os.ReadFile(calls); err != nil || string(data) != "x" {
		t.Fatalf("initial probe execution = %q, %v", data, err)
	}
	if _, err := os.Stat(probe.cachePath(item.ID)); err != nil {
		t.Fatalf("initial probe cache persistence = %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	restarted := New(executable)
	restarted.ConfigureCache(root)
	readers := []struct {
		name  string
		probe *Probe
	}{{"memory", probe}, {"restarted", restarted}}
	for _, reader := range readers {
		if facts, found := reader.probe.CachedScanFacts(item); !found || facts.Video.Codec != "h264" {
			t.Fatalf("%s scan facts = %#v, %v", reader.name, facts, found)
		}
		assertStaleScanRejected(t, item, reader.probe.CachedScanFacts)
		if _, found := reader.probe.CachedFacts(item); found {
			t.Fatalf("%s live lookup accepted a removed source", reader.name)
		}
	}
	if data, err := os.ReadFile(calls); err != nil || string(data) != "x" {
		t.Fatalf("scan lookups launched FFprobe: %q %v", data, err)
	}
}
