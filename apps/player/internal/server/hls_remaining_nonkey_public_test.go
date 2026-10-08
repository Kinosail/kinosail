package server_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/playback"
)

// Actual public preparation must distinguish a requested presentation origin
// from its preceding independent decode key. Cold fallback does not prove this.
func TestRemainingNonKeyPreparationReachesReady(t *testing.T) {
	if os.Getenv("KINOSAIL_COPIED_RECOVERY_MEDIA") != "1" {
		t.Skip("The designated pinned-codec hosted job runs this public regression")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("nonkey fixture: pinned FFmpeg unavailable")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal("nonkey fixture: pinned FFprobe unavailable")
	}
	for _, offset := range []float64{12, 12.5} {
		t.Run(fmt.Sprintf("offset-%.1f", offset), func(t *testing.T) {
			remainingNonKeyPreparation(t, ffmpeg, ffprobe, offset)
		})
	}
}

func remainingNonKeyPreparation(t *testing.T, ffmpeg, ffprobe string, offset float64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	media := t.TempDir()
	source := filepath.Join(media, "Nonkey.mp4")
	remainingNonKeyGenerate(t, ctx, ffmpeg, source)
	remainingNonKeyQualify(t, ctx, ffmpeg, ffprobe, source)
	observed := remainingNonKeyObserve(t)
	before, err := os.ReadFile(source)
	if err != nil || len(before) == 0 || len(before) > 64<<20 {
		t.Fatal("nonkey fixture byte bound")
	}
	handler, id := remainingNonKeyHandler(t, server.Config{Lifecycle: ctx, MediaDir: media, FFprobe: ffprobe}, cancel, source, ffmpeg, before)
	selected := remainingNonKeySelectedSource(t, handler, id, offset)
	t.Logf("nonkey fixture qualified offset=%.1f sha=%x frames=768 idr-keys=16 eof=32", offset, sha256.Sum256(before))
	state := remainingNonKeyPoll(t, handler, id, selected, offset, observed, sha256.Sum256(before))
	after, err := os.ReadFile(source)
	if err != nil || sha256.Sum256(after) != sha256.Sum256(before) {
		t.Fatal("nonkey preparation mutated its source")
	}
	t.Logf("nonkey regression offset=%.1f phase=public-preparation state=%s fixture=%x frames=768 idr-keys=16", offset, state, sha256.Sum256(before))
}

func remainingNonKeyHandler(t *testing.T, config server.Config, cancel context.CancelFunc, source, ffmpeg string, before []byte) (http.Handler, string) {
	t.Helper()
	tools, cache := t.TempDir(), t.TempDir()
	owned, adapter := filepath.Join(tools, "owned-pids"), filepath.Join(tools, "ffmpeg")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	body := "#!/bin/sh\nset -eu\nprintf '%s\\n' $$ >> " + quote(owned) + "\nexec " + quote(ffmpeg) + " \"$@\"\n"
	//nolint:gosec // Owned executable adapter.
	if os.WriteFile(adapter, []byte(body), 0o700) != nil {
		t.Fatal("nonkey owned codec adapter")
	}
	config.DataDir, config.CacheDir, config.FFmpeg = t.TempDir(), cache, adapter
	handler, id := formatTestItem(t, config)
	t.Cleanup(func() {
		cancel()
		settled, release := context.WithTimeout(context.Background(), 3*time.Second)
		defer release()
		if data, readErr := os.ReadFile(owned); readErr != nil || len(data) == 0 || len(data) > 16<<10 {
			t.Fatal("nonkey owned codec receipt missing or outside bound")
		}
		copiedRecoveryJoinedCodecs(t, settled, owned)
		after, readErr := os.ReadFile(source)
		if readErr != nil || sha256.Sum256(after) != sha256.Sum256(before) {
			t.Fatal("nonkey preparation mutated its source during failure or shutdown")
		}
	})
	return handler, id
}

func remainingNonKeySelectedSource(t *testing.T, handler http.Handler, id string, offset float64) string {
	t.Helper()
	var plan struct {
		Compatible     string
		CompatiblePlan playback.PlaybackPlan
	}
	mustJSON(t, apiCall(t, handler, "", http.MethodGet,
		"/api/v1/items/"+id+"/playback?videoCodecs=h264&audioCodecs=aac", nil), &plan)
	if plan.CompatiblePlan.Mode != "remux" || !strings.HasSuffix(plan.Compatible, "/index.m3u8") {
		t.Fatal("nonkey public recipe qualification")
	}
	return strings.TrimSuffix(plan.Compatible, "/index.m3u8") +
		fmt.Sprintf("-o%d/index.m3u8", int(offset*1000))
}

func remainingNonKeyPoll(t *testing.T, handler http.Handler, id, selected string, offset float64, observed *remainingNonKeyLog, fixture [32]byte) string {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	requestID := ""
	for {
		state := remainingNonKeyPrepareState(t, handler, id, selected, &requestID)
		if state == "ready" {
			return state
		}
		remainingNonKeyPending(t, observed, requestID, state, offset, fixture, deadline)
		time.Sleep(20 * time.Millisecond)
	}
}

func remainingNonKeyPrepareState(t *testing.T, handler http.Handler, id, selected string, requestID *string) string {
	t.Helper()
	var preparation struct{ State string }
	response := apiCall(t, handler, "", http.MethodPost,
		"/api/v1/items/"+id+"/playback-prepare", map[string]any{"source": selected})
	if response.Code != http.StatusAccepted {
		t.Fatalf("nonkey preparation status=%d", response.Code)
	}
	if *requestID == "" {
		*requestID = response.Header().Get("X-Request-ID")
		if !remainingNonKeyValidRequestID(*requestID) {
			t.Fatal("nonkey public request identity")
		}
	}
	mustJSON(t, response, &preparation)
	return preparation.State
}

func remainingNonKeyPending(t *testing.T, observed *remainingNonKeyLog, requestID, state string, offset float64, fixture [32]byte, deadline time.Time) {
	t.Helper()
	if phase, failure := observed.rejection(requestID); failure {
		t.Fatalf("nonkey regression offset=%.1f phase=%s internal-state=unavailable public-state=%s expected=ready fixture=%x", offset, phase, state, fixture)
	}
	if state != "queued" {
		t.Fatalf("nonkey regression offset=%.1f phase=public-preparation state=%s expected=ready fixture=%x", offset, state, fixture)
	}
	if time.Now().After(deadline) {
		t.Fatalf("nonkey preparation deadline offset=%.1f state=%s", offset, state)
	}
}
