package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadStateFileRejectsReplacementAfterPathCheck(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "settings.json")
	replacement := filepath.Join(directory, "replacement.json")
	original := filepath.Join(directory, "original.json")
	if err := os.WriteFile(path, []byte(`{"name":"Kinosail"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(replacement, []byte(`{"name":"private"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readStateFileWith(path, maxFileSize, os.Lstat, func(name string) (*os.File, error) {
		if err := os.Rename(name, original); err != nil {
			return nil, err
		}
		if err := os.Symlink(replacement, name); err != nil {
			return nil, err
		}
		return os.Open(name)
	}); err == nil {
		t.Fatal("state-file replacement race was accepted")
	}
}
