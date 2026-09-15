package server

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/workload"
)

func TestScheduledBackupRejectsInvalidSubtitleSettingsWithoutPublishingArchive(t *testing.T) {
	dataDir, backupDir := t.TempDir(), t.TempDir()
	original := []byte(`{"subtitleLanguages":[]}`)
	if err := os.WriteFile(filepath.Join(dataDir, "settings.json"), original, 0o600); err != nil {
		t.Fatal(err)
	}
	manager := newBackupManager(t.Context(), dataDir, backupDir, "container-test-backup-key", time.Hour, 7, workload.New(1))
	if err := manager.WriteNow(); err == nil {
		t.Fatal("semantic-invalid settings were backed up")
	}
	if files, err := manager.Files(); err != nil || len(files) != 0 {
		t.Fatalf("rejected backup published files = %v, %v", files, err)
	}
	current, err := os.ReadFile(filepath.Join(dataDir, "settings.json"))
	if err != nil || !bytes.Equal(current, original) {
		t.Fatalf("rejected backup changed settings: %q, %v", current, err)
	}
}
