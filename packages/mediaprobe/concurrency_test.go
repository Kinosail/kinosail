package mediaprobe

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestProbeDecorationUsesAvailableHardware(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 3 {
		t.Skip("parallel probing reserves one logical CPU")
	}
	root := t.TempDir()
	starts, executable := filepath.Join(root, "starts"), filepath.Join(root, "ffprobe")
	script := fmt.Sprintf(`#!/bin/sh
printf x >> %q
attempt=0
while [ "$(wc -c < %q)" -lt 2 ] && [ "$attempt" -lt 500 ]; do
  sleep 0.01
  attempt=$((attempt + 1))
done
[ "$(wc -c < %q)" -ge 2 ] || exit 1
printf '%%s' '{"format":{"tags":{"title":"Probed"}}}'
`, starts, starts, starts)
	if err := os.WriteFile(executable, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(executable, 0o700); err != nil { //nolint:gosec // The temporary fixture must be executable.
		t.Fatal(err)
	}
	items := []library.Item{{ID: "one", Kind: "audio", Path: filepath.Join(root, "one.flac")}, {ID: "two", Kind: "audio", Path: filepath.Join(root, "two.flac")}}
	items = New(executable).Decorate(t.Context(), items, Enrichment{})
	if items[0].Title != "Probed" || items[1].Title != "Probed" {
		t.Fatalf("probed titles = %q, %q", items[0].Title, items[1].Title)
	}
}
