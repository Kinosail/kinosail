package mediaprobe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestFactsReuseMemoryAndDiskWithoutKeyframeAnalysis(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	media, executable, calls := filepath.Join(root, "video.mkv"), filepath.Join(root, "ffprobe"), filepath.Join(root, "calls")
	if err := os.WriteFile(media, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeProbeScript(t, executable, "#!/bin/sh\nprintf x >> '"+calls+"'\nprintf '%s' '{\"format\":{\"duration\":\"120\"},\"chapters\":[{\"start_time\":\"0\",\"end_time\":\"20\",\"tags\":{\"title\":\"Opening Credits\"}}]}'\n")
	item := library.Item{ID: "video", Kind: "video", Path: media}
	probe := New(executable)
	probe.ConfigureCache(root)
	for range 2 {
		result := probe.Facts(t.Context(), item)
		if result.Duration != 120 || len(result.Markers) == 0 || result.RandomAccess != nil {
			t.Fatalf("facts = %#v", result)
		}
	}
	restored := New(executable)
	restored.ConfigureCache(root)
	if result := restored.Facts(t.Context(), item); result.Duration != 120 || result.RandomAccess != nil {
		t.Fatalf("restored facts = %#v", result)
	}
	data, err := os.ReadFile(calls)
	if err != nil || strings.TrimSpace(string(data)) != "x" {
		t.Fatalf("probe calls = %q, error = %v", data, err)
	}
}
