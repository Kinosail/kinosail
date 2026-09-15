package mediaprobe

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func BenchmarkProbeDecoration(b *testing.B) {
	b.StopTimer()
	root := b.TempDir()
	executable := filepath.Join(root, "ffprobe")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nsleep 0.01\nprintf '%s' '{\"format\":{\"tags\":{\"title\":\"Probed\"}}}'\n"), 0o600); err != nil {
		b.Fatal(err)
	}
	if err := os.Chmod(executable, 0o700); err != nil { //nolint:gosec // The temporary fixture must be executable.
		b.Fatal(err)
	}
	items := make([]library.Item, 16)
	for position := range items {
		id := strconv.Itoa(position)
		items[position] = library.Item{ID: id, Kind: "audio", Path: filepath.Join(root, id+".flac")}
	}
	b.StartTimer()
	for b.Loop() {
		result := New(executable).Decorate(b.Context(), append([]library.Item(nil), items...), Enrichment{})
		if result[len(result)-1].Title != "Probed" {
			b.Fatal("last audio item was not probed")
		}
	}
}
