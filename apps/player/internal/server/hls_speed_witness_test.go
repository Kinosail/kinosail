package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHLSSpeedFailureWitness(t *testing.T) {
	cache := t.TempDir()
	recipe := filepath.Join(cache, "private-recipe")
	if err := os.MkdirAll(filepath.Join(recipe, "180p"), 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(name, value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(recipe, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(".source", "private-source-and-policy")
	write(".copy-timeline", `{"Keys":[{"PTS":0},{"PTS":2}],"Clock":0,"Policy":"private-policy"}`)
	write("180p/index.m3u8", "#EXTM3U\n#EXTINF:2,\nsegment-00003.m4s\n")
	before := speedFailureCacheFacts(cache, "180p/segment-00003.m4s")
	assertSpeedWitness(t, before, "missing", 2, "present")
	write("180p/segment-00003.m4s", "private-media")
	after := speedFailureCacheFacts(cache, "180p/segment-00003.m4s")
	assertSpeedWitness(t, after, "regular", 2, "present")
	if after[0].Files["segment"].Bytes != 13 || after[0].Files["segment"].SHA256 == "" {
		t.Fatalf("present-segment witness: %+v", after)
	}
	if err := os.Remove(filepath.Join(recipe, "180p/segment-00003.m4s")); err != nil {
		t.Fatal(err)
	}
	removed := speedFailureCacheFacts(cache, "180p/segment-00003.m4s")
	assertSpeedWitness(t, removed, "missing", 2, "present")
	data, err := json.Marshal(after)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{cache, "private-recipe", "private-source", "private-policy", "private-media", "EXTM3U"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("witness leaked %q", secret)
		}
	}
}

func TestHLSSpeedFailureWitnessRejectsUnsafeFilesAndInputs(t *testing.T) {
	cache := t.TempDir()
	recipe := filepath.Join(cache, "recipe")
	if err := os.MkdirAll(filepath.Join(recipe, "180p"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(recipe, ".source"), []byte("binding"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("private-outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(recipe, "180p/segment-00003.m4s")); err != nil {
		t.Fatal(err)
	}
	facts := speedFailureCacheFacts(cache, "180p/segment-00003.m4s")
	if len(facts) != 1 || facts[0].Files["segment"].State != "nonregular" || facts[0].Files["segment"].SHA256 != "" {
		t.Fatalf("linked witness: %+v", facts)
	}
	for _, target := range []string{"", "../segment-00003.m4s", "+180p/segment-00003.m4s", "0180p/segment-00003.m4s", "99999p/segment-00003.m4s", "180p/../segment-00003.m4s", "180p/segment-0000X.m4s", strings.Repeat("x", 1000)} {
		if len(speedFailureCacheFacts(cache, target)) != 0 {
			t.Fatal("unsafe target admitted")
		}
	}
}

// Observations are not admission decisions. Reads are bounded and rooted in
// the owned fixture cache; a replaced file is explicitly marked unstable.
func TestHLSSpeedFailureWitnessBoundsCapturedLogs(t *testing.T) {
	output := &copiedLogBuffer{limit: 64}
	if n, err := output.Write([]byte(strings.Repeat("a", 128))); err != nil || n != 128 {
		t.Fatal("logger write contract changed")
	}
	if n, err := output.Write([]byte("tail")); err != nil || n != 4 {
		t.Fatal("second logger write contract changed")
	}
	if data := output.snapshot(); len(data) != 64 || !strings.HasSuffix(data, "tail") {
		t.Fatalf("captured logs not capped: bytes=%d", len(data))
	}
}

// The subprocess deliberately fails the original HTTP oracle. Its parent proves
// the failure evidence is present, private and cannot convert the failure to pass.
func TestHLSSpeedFailureWitnessHTTP(t *testing.T) {
	if os.Getenv("KINOSAIL_SPEED_WITNESS_CHILD") == "1" {
		speedTestGET(t, t.Context(), speedWitnessHTTPHandler(t), "/hls/private-item/p/private-recipe/180p/segment-00003.m4s?playbackSession=hls-speed-fixture")
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, executable, "-test.run=^TestHLSSpeedFailureWitnessHTTP$", "-test.v")
	child.Env = append(os.Environ(), "KINOSAIL_SPEED_WITNESS_CHILD=1")
	data, err := child.CombinedOutput()
	if err == nil || ctx.Err() != nil {
		t.Fatalf("controlled delivery failure not retained: %v", err)
	}
	for _, marker := range []string{"failed-request cache observations", "target_valid=true", `"segment":{"State":"regular"`, "retained log capture: bytes=0 phase_records=0", "segment-00003.m4s", "= 404"} {
		if !bytes.Contains(data, []byte(marker)) {
			t.Fatalf("failure witness missing %s: %s", marker, data)
		}
	}
	for _, secret := range []string{"private-response-body", "private-item", "private-recipe", "private-media", "private-binding", "playbackSession", "hls-speed-fixture"} {
		if bytes.Contains(data, []byte(secret)) {
			t.Fatal("HTTP failure leaked private fixture value")
		}
	}
}

func speedWitnessHTTPHandler(t *testing.T) *speedDiagnosticHandler {
	t.Helper()
	cache := t.TempDir()
	recipe := filepath.Join(cache, "private-recipe")
	if err := os.MkdirAll(filepath.Join(recipe, "180p"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{".source": "private-binding", "180p/segment-00003.m4s": "private-media"} {
		if err := os.WriteFile(filepath.Join(recipe, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return &speedDiagnosticHandler{cache: cache, logs: captureCopiedLogs(t), Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/hls/private-item/p/private-recipe/180p/segment-00003.m4s" || r.URL.RawQuery != "playbackSession=hls-speed-fixture" {
			t.Fatal("observation changed the original request")
		}
		http.Error(w, "private-response-body", http.StatusNotFound)
	})}
}

func TestHLSSpeedFailureWitnessProjectsOnlyKnownOperationFields(t *testing.T) {
	entry := map[string]any{"msg": "HLS transcode failed", "request_id": "synthetic-request", "playback_session": "hls-speed-fixture", "mode": "remux", "error": "private-error", "phase": "private-phase", "state": "private-state", "duration_ms": float64(12)}
	data, _ := json.Marshal(speedFailureOperationFields(entry, "synthetic-request"))
	if !bytes.Contains(data, []byte(`"request_matches":true`)) || !bytes.Contains(data, []byte(`"session_matches":true`)) || !bytes.Contains(data, []byte(`"mode":"remux"`)) {
		t.Fatalf("known causal fields missing: %s", data)
	}
	if bytes.Contains(data, []byte("private")) || bytes.Contains(data, []byte("synthetic-request")) {
		t.Fatal("private log fields leaked")
	}
	entry["mode"] = "private-mode"
	entry["duration_ms"] = "private-duration"
	entry["failure_class"] = "private-class"
	data, _ = json.Marshal(speedFailureOperationFields(entry, "other-request"))
	if bytes.Contains(data, []byte("private")) || bytes.Contains(data, []byte(`"request_matches":true`)) {
		t.Fatal("unknown operation fields admitted")
	}
}

func TestHLSSpeedFailureWitnessTimelineBounds(t *testing.T) {
	cache := t.TempDir()
	recipe := filepath.Join(cache, "recipe")
	if err := os.Mkdir(recipe, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(recipe, ".source"), []byte("binding"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(recipe, ".copy-timeline"), []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	facts := speedFailureCacheFacts(cache, "180p/segment-00003.m4s")
	if facts[0].Clock != "unavailable" || facts[0].TimelineKeys != -1 {
		t.Fatalf("malformed timeline fabricated facts: %+v", facts)
	}
	if err := os.WriteFile(filepath.Join(recipe, ".copy-timeline"), []byte(strings.Repeat("x", (256<<10)+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	facts = speedFailureCacheFacts(cache, "180p/segment-00003.m4s")
	if facts[0].Files["timeline"].State != "oversized" || facts[0].Clock != "unavailable" {
		t.Fatalf("oversized timeline admitted: %+v", facts)
	}
}

func assertSpeedWitness(t *testing.T, facts []speedCacheFact, state string, keys int, clock string) {
	t.Helper()
	if len(facts) != 1 {
		t.Fatalf("recipe witness count=%d", len(facts))
	}
	fact := facts[0]
	if fact.Files["segment"].State != state || fact.TimelineKeys != keys || fact.Clock != clock {
		t.Fatalf("unexpected bounded witness: %+v", fact)
	}
}
