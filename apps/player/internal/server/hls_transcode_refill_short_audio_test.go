package server_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Replay the original warm-up budget defect through the real encoder. A zero
// encoder exit must not authorize publication of an incomplete AAC packet grid.
func transcodeRefillRejectShortAudio(t *testing.T, ctx context.Context, root, prefix, owned, variant, fragment, source, adapter string, beforeOwned int, logs *copiedLogBuffer) int {
	t.Helper()
	original := transcodeRefillRead(t, adapter)
	complete := filepath.Join(filepath.Dir(adapter), "ffmpeg-complete")
	if err := os.WriteFile(complete, original, 0o700); err != nil { //nolint:gosec // Owned executable adapter in the disposable fixture.
		t.Fatal("cannot retain the complete encoder adapter")
	}
	defer func() {
		if err := os.WriteFile(adapter, original, 0o700); err != nil { //nolint:gosec // Restore the same owned executable adapter.
			t.Error("cannot restore the complete encoder adapter")
		}
	}()
	script := `#!/bin/bash
set -eu
args=()
duration=0
replaced=0
for value in "$@"; do
  if [ "$duration" = 1 ]; then
    args+=("4.5")
    duration=0
    replaced=$((replaced+1))
  else
    args+=("$value")
    if [ "$value" = "-t" ]; then duration=1; fi
  fi
done
test "$replaced" = 1
exec '` + strings.ReplaceAll(complete, "'", "'\"'\"'") + `' "${args[@]}"
`
	if err := os.WriteFile(adapter, []byte(script), 0o700); err != nil { //nolint:gosec // Fixed bounded duration control on the test-owned codec adapter.
		t.Fatal("cannot prepare the short audio output control")
	}
	joined := transcodeRefillRejectStaged(t, ctx, root, prefix, owned, variant, fragment, source, beforeOwned, logs, "shortAudio", transcodeRefillShortAudioFields)
	matches := 0
	for _, producer := range transcodeRefillProducers(t, filepath.Dir(adapter)) {
		if duration := producer["-t"]; len(duration) == 1 && duration[0] == "4.5" {
			matches++
		}
	}
	if matches != 1 {
		t.Error("short audio control did not execute exactly one shortened encoder")
	}
	transcodeRefillLog(t, map[string]any{"shortAudioEncoder": true, "duration": 4.5, "shortenedProducers": matches})
	return joined
}

func transcodeRefillShortAudioFields(record map[string]any) bool {
	clock := transcodeRefillClockFields(record)
	return record["msg"] == "HLS transcode failed" && transcodeRefillFailureOperation(record) == "aac-packet-duration" && clock["measured"] == false && clock["packet_pts"] == float64(742400) && clock["packet_duration"] == float64(960)
}
