package backup_test

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlainRestoreStartsAFreshActivityJournalFromOlderArchives(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	destination := t.TempDir()
	writeBackupFile(t, filepath.Join(destination, "audit.jsonl"), "{\"action\":\"before.restore\"}\n")
	writeBackupFile(t, filepath.Join(destination, "audit_key.json"), `{"key":"kept"}`)
	archive := tarArchive(t, []archiveEntry{{name: "settings.json", data: `{"name":"Older"}`}, {name: "audit.jsonl", data: "{\"action\":\"unverifiable\"}\n"}})
	if err := testArchive.Restore(bytes.NewReader(archive), destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, "audit.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("plain restore kept an activity journal it cannot verify: %v", err)
	}
	if key, err := os.ReadFile(filepath.Join(destination, "audit_key.json")); err != nil || string(key) != `{"key":"kept"}` {
		t.Fatalf("plain restore changed the activity journal key: %q, %v", key, err)
	}
	if settings, err := os.ReadFile(filepath.Join(destination, "settings.json")); err != nil || string(settings) != `{"name":"Older"}` {
		t.Fatalf("restored settings = %q, %v", settings, err)
	}
	if lines := strings.Count(logs.String(), "\n"); lines != 1 || !strings.Contains(logs.String(), "level=WARN") || !strings.Contains(logs.String(), "outcome=activity_journal_not_restored") || strings.Contains(logs.String(), "unverifiable") {
		t.Fatalf("restore logs = %q", logs.String())
	}
}
