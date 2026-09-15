package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRollbackRestoreRestoresOriginals(t *testing.T) {
	dataDir := t.TempDir()
	installed := filepath.Join(dataDir, "settings.json")
	original := filepath.Join(dataDir, ".original")
	if err := os.WriteFile(installed, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(original, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := rollbackRestore(dataDir, []string{"settings.json"}, map[string]string{"settings.json": original}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(installed)
	if err != nil || string(data) != "old" {
		t.Fatalf("restored content = %q, %v", data, err)
	}
	if _, err := os.Stat(original); !os.IsNotExist(err) {
		t.Fatalf("original staging file remains: %v", err)
	}
}

func TestRollbackRestoreReportsCleanupFailures(t *testing.T) {
	dataDir := t.TempDir()
	installed := filepath.Join(dataDir, "settings.json")
	if err := os.Mkdir(installed, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installed, "child"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := rollbackRestore(dataDir, []string{"settings.json"}, map[string]string{"profiles.json": filepath.Join(dataDir, "missing")})
	if err == nil {
		t.Fatal("rollback failures were ignored")
	}
	if _, statErr := os.Stat(installed); statErr != nil {
		t.Fatalf("failed cleanup removed the installed directory: %v", statErr)
	}
}
