package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestMarkerProbeKeepsInterruptedInspectionRetryable(t *testing.T) {
	root := t.TempDir()
	ffprobe, killed := filepath.Join(root, "ffprobe"), filepath.Join(root, "killed")
	script := "#!/bin/sh\nif [ ! -e '" + killed + "' ]; then : > '" + killed + "'; kill -KILL $$; fi\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\"}],\"format\":{\"duration\":\"30\"}}'\n"
	if err := os.WriteFile(ffprobe, []byte(script), 0o700); err != nil { //nolint:gosec // Test executables must be runnable.
		t.Fatal(err)
	}
	item := library.Item{ID: "film", Kind: "video", Path: filepath.Join(root, "film.mkv")}
	if err := os.WriteFile(item.Path, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	inspect := markerProbe(newMediaProbe(ffprobe))
	if media := inspect(t.Context(), item); !media.Interrupted || media.Duration != 0 {
		t.Fatalf("killed inspection = %#v", media)
	}
	if media := inspect(t.Context(), item); media.Interrupted || media.Duration != 30 {
		t.Fatalf("retried inspection = %#v", media)
	}
}
