package servertest

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
)

type lockedLog struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (log *lockedLog) Write(data []byte) (int, error) {
	log.mu.Lock()
	defer log.mu.Unlock()
	return log.buffer.Write(data)
}

func (log *lockedLog) records(message string) []map[string]any {
	log.mu.Lock()
	defer log.mu.Unlock()
	var records []map[string]any
	for line := range strings.SplitSeq(log.buffer.String(), "\n") {
		var record map[string]any
		if json.Unmarshal([]byte(line), &record) == nil && record["msg"] == message {
			records = append(records, record)
		}
	}
	return records
}

func (log *lockedLog) contains(value string) bool {
	log.mu.Lock()
	defer log.mu.Unlock()
	return strings.Contains(log.buffer.String(), value)
}

func AssertPersistentBackupFailureLogsOnceAndRecovers(t *testing.T, fixture BackupAPIFixture) {
	t.Helper()
	output := &lockedLog{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	root := t.TempDir()
	blocker := filepath.Join(root, "private-parent")
	if err := os.WriteFile(blocker, []byte("not a folder"), 0o600); err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(blocker, "backups")
	handler := fixture.New(BackupAPIConfig{Lifecycle: t.Context(), DataDir: t.TempDir(), BackupDir: backupDir, Key: "container-test-backup-key", Interval: 20 * time.Millisecond})
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	waitForRecords(t, output, "automatic backup failed", 4)
	failures := output.records("automatic backup failed")
	warnings := 0
	for _, record := range failures {
		if record["level"] == "WARN" {
			warnings++
		} else if record["level"] != "DEBUG" {
			t.Fatalf("repeated failure level = %v", record)
		}
		if record["operation"] != "backup.automatic" || record["failure_class"] != "invalid-path" {
			t.Fatalf("failure record lacks class: %v", record)
		}
	}
	if warnings != 1 || output.contains(root) {
		t.Fatalf("warnings = %d, records = %v, path leaked = %v", warnings, failures, output.contains(root))
	}
	status := APICall(t, handler, owner.Value, http.MethodGet, "/api/v1/backups", nil)
	AssertAPIBody(t, status, http.StatusOK, `"lastError":"The backup folder path is not a folder Kinosail can use."`)
	maintenance := APICall(t, handler, owner.Value, http.MethodGet, "/api/v1/maintenance", nil)
	AssertAPIBody(t, maintenance, http.StatusOK, `"attention":true`, `"state":"error"`)
	page := fixture.Get(t, handler, "/settings/backups", owner)
	AssertAPIBody(t, page, http.StatusOK, "Last error: The backup folder path is not a folder Kinosail can use.")
	if strings.Contains(status.Body.String()+page.Body.String(), "private-parent/backups:") {
		t.Fatalf("status leaked the failing path: %q", status.Body.String())
	}
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	waitForRecords(t, output, "automatic backup recovered", 1)
	if recovered := output.records("automatic backup recovered"); recovered[0]["level"] != "INFO" || recovered[0]["previous_failure_class"] != "invalid-path" {
		t.Fatalf("recovery record = %v", recovered)
	}
}

func waitForRecords(t *testing.T, output *lockedLog, message string, count int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for len(output.records(message)) < count {
		if time.Now().After(deadline) {
			t.Fatalf("waited for %d %q records, got %v", count, message, output.records(message))
		}
		time.Sleep(10 * time.Millisecond)
	}
}
