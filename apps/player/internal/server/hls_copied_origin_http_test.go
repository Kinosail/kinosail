package server_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/playback"
)

// Indexed source timestamps are absolute. A stream whose first source IDR is
// nonzero cannot certify a zero-origin request by treating file-relative time
// as that source timestamp. Ordinary compatible playback remains available.
func TestRealCopiedHLSHTTPPositiveSourceOriginRejectsZeroOriginCertification(t *testing.T) {
	ffmpeg, ffprobe := copiedHLSTools(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	media, source := positiveCopiedSource(t, ctx, ffmpeg, ffprobe)
	cache := t.TempDir()
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	originalHash := sha256.Sum256(original)
	marker := filepath.Join(cache, "unrelated-owned-cache")
	if err := os.WriteFile(marker, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	logs := captureCopiedLogs(t)
	retainCopiedFailureFacts(t, ffmpeg, ffprobe, source, cache, logs)
	handler, id := formatTestItem(t, server.Config{Lifecycle: ctx, MediaDir: media, DataDir: t.TempDir(), CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe})
	var plan struct{ Compatible string }
	mustJSON(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback?videoCodecs=h264&audioCodecs=aac", nil), &plan)
	response := prepareSource(t, handler, id, plan.Compatible)
	assertPreparationState(t, response, http.StatusAccepted, "queued")
	awaitCopiedLog(t, logs, "HLS startup preparation", response.Header().Get("X-Request-ID"), "unavailable")
	directory := filepath.Join(cache, playback.HLSRecipeKey(id, playback.HLSRecipe{Mode: "remux"}))
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("uncertified source allocated media/cache: %v", err)
	}
	unchanged, err := os.ReadFile(source)
	if err != nil || sha256.Sum256(unchanged) != originalHash {
		t.Fatal("certification changed source")
	}
	untouched, err := os.ReadFile(marker)
	if err != nil || !bytes.Equal(untouched, []byte("unchanged")) {
		t.Fatal("certification changed unrelated cache")
	}
	// A rejected speculative index is not a refusal of ordinary public playback.
	master := speedTestGET(t, ctx, handler, plan.Compatible)
	variants := speedTestURIs(master)
	if len(variants) != 1 {
		t.Fatalf("fallback rendition count=%d", len(variants))
	}
	base := plan.Compatible[:strings.LastIndex(plan.Compatible, "/")+1]
	assertIndependentHLSRendition(t, ctx, handler, base+variants[0], ffmpeg, ffprobe)
}

func positiveCopiedSource(t *testing.T, ctx context.Context, ffmpeg, ffprobe string) (string, string) {
	t.Helper()
	media := t.TempDir()
	source := filepath.Join(media, "Shifted.mkv")
	generate := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=12:duration=12", "-c:v", "libx264", "-preset", "ultrafast", "-g", "24", "-keyint_min", "24", "-sc_threshold", "0", "-bf", "0", "-an", "-output_ts_offset", "5", source) //nolint:gosec // Fixed generated owned fixture and discovered tool.
	if output, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("generate nonzero-origin fixture: %v: %s", err, output)
	}
	probe := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "v:0", "-read_intervals", "%+#1", "-show_entries", "packet=pts_time", "-of", "csv=p=0", source) //nolint:gosec // Fixed metadata arguments and owned source.
	packet, err := probe.Output()
	if err != nil {
		t.Fatal(err)
	}
	first, err := strconv.ParseFloat(strings.TrimSpace(string(packet)), 64)
	if err != nil || first < 4.999 || first > 5.001 {
		t.Fatalf("generated source first PTS must be5s: %q", packet)
	}
	return media, source
}
