package server_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestSubtitleOperationLifecycleDiagnosticsAndMetadataExcludePrivatePayloads(t *testing.T) {
	config, target, _, _ := subtitleOperationAudioConfig(t, false)
	handler := server.New(config)
	base := "/api/v1/subtitle-library/" + firstSubtitleInventoryID(t, handler)
	before := subtitleActionRead(t, handler, base)
	var logs subtitleOperationLogCapture
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	private := "R06_PRIVATE_SUBTITLE_PAYLOAD_SENTINEL"
	prepared := prepareSubtitleOperation(t, handler, base, "apply")
	input, _ := json.Marshal(map[string]any{"language": "en", "fingerprint": before.Fingerprint, "text": "1\n00:00:01,000 --> 00:00:02,000\n" + private + "\n"})
	assertSubtitleOperationAccepted(t, activateSubtitleOperation(t, handler, base+"/apply", string(input), []string{prepared.ID}), prepared.ID)
	completed := waitSubtitleOperation(t, handler, prepared.ID)
	if completed.Outcome != "success" || completed.Status != http.StatusOK {
		t.Fatalf("diagnostic Save control = %+v", completed)
	}
	rejected := activateSubtitleOperation(t, handler, base+"/restore", `{"language":"en"}`, []string{strings.Repeat("A", 64)})
	if rejected.Code != http.StatusBadRequest {
		t.Fatalf("diagnostic malformed-header control = %d", rejected.Code)
	}
	text := waitSubtitleOperationCompletionLog(t, &logs)
	for _, forbidden := range []string{private, target, config.DataDir, config.CacheDir, "fingerprint", string(input)} {
		if strings.Contains(text, forbidden) {
			t.Fatal("operation diagnostics leaked private payload or storage metadata")
		}
	}
	assertSubtitleOperationLifecycleLog(t, text, "INFO", "subtitle operation started", prepared.ID, "apply")
	assertSubtitleOperationLifecycleLog(t, text, "INFO", "subtitle operation completed", prepared.ID, "apply")
	assertSubtitleOperationLifecycleLog(t, text, "WARN", "subtitle operation rejected", "", "restore")
	path := filepath.Join(config.DataDir, "subtitle_operations.json")
	metadata, err := os.ReadFile(path)
	if err != nil || len(metadata) > 96*1024 {
		t.Fatal("durable receipt metadata is missing or unbounded")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm()&0o077 != 0 {
		t.Fatal("receipt metadata is not owner-readable-only")
	}
	for _, forbidden := range []string{private, target, config.DataDir, config.CacheDir, `"text"`, `"waveform"`, `"speech"`} {
		if bytes.Contains(metadata, []byte(forbidden)) {
			t.Fatal("durable operation metadata retained private payload or audio")
		}
	}
}

func waitSubtitleOperationCompletionLog(t *testing.T, logs *subtitleOperationLogCapture) string {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if text := logs.String(); strings.Contains(text, `"msg":"subtitle operation completed"`) {
			return text
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("completed public receipt did not produce its lifecycle diagnostic")
	return ""
}

type subtitleOperationLogCapture struct {
	mu sync.Mutex
	bytes.Buffer
}

func (capture *subtitleOperationLogCapture) Write(data []byte) (int, error) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return capture.Buffer.Write(data)
}

func (capture *subtitleOperationLogCapture) String() string {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return capture.Buffer.String()
}

func assertSubtitleOperationLifecycleLog(t *testing.T, logs, level, message, id, action string) {
	t.Helper()
	for _, line := range strings.Split(logs, "\n") {
		var entry map[string]any
		if json.Unmarshal([]byte(line), &entry) != nil || entry["msg"] != message {
			continue
		}
		requestID, bounded := entry["request_id"].(string)
		if entry["level"] == level && entry["action"] == action && bounded && len(requestID) > 0 && len(requestID) <= 128 && (id == "" || entry["operation_id"] == id) {
			return
		}
	}
	t.Fatalf("missing structured %s lifecycle diagnostic for %s/%s", level, action, message)
}
