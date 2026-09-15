package backup_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreReportsDestinationFilesystemFailures(t *testing.T) {
	archive := tarArchive(t, []archiveEntry{{name: "settings.json", data: `{}`}})
	file := filepath.Join(t.TempDir(), "destination")
	writeBackupFile(t, file, "not a directory")
	if err := testArchive.Restore(bytes.NewReader(archive), file); err == nil {
		t.Fatal("non-directory restore destination was accepted")
	}

	destination := t.TempDir()
	blocked := filepath.Join(destination, "settings.json")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	writeBackupFile(t, filepath.Join(blocked, "child"), "keep directory non-empty")
	if err := testArchive.Restore(bytes.NewReader(archive), destination); err == nil {
		t.Fatal("restore over a non-empty directory succeeded")
	}

	absentArchive := tarArchive(t, []archiveEntry{{name: "profiles.json", data: `[]`}})
	if err := testArchive.Restore(bytes.NewReader(absentArchive), destination); err == nil {
		t.Fatal("failure removing absent managed state was ignored")
	}

	rollback := t.TempDir()
	writeBackupFile(t, filepath.Join(rollback, "settings.json"), `{"name":"Before"}`)
	if err := os.Mkdir(filepath.Join(rollback, "profiles.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := testArchive.Restore(bytes.NewReader(archive), rollback); err == nil {
		t.Fatal("restore with an invalid absent target succeeded")
	}
	if data, err := os.ReadFile(filepath.Join(rollback, "settings.json")); err != nil || string(data) != `{"name":"Before"}` {
		t.Fatalf("failed restore changed existing state: %q, %v", data, err)
	}
}
