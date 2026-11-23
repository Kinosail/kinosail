package documentdb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDatabaseOpensExactlyTheConfiguredFolder(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	for _, folder := range []string{filepath.Join(root, "Kinosail #1 ?50% data"), "relative data"} {
		store, err := Open(folder, false, Config{Documents: []string{"settings.json"}})
		if err != nil {
			t.Fatalf("open %q: %v", folder, err)
		}
		if err := store.SaveJSON("settings.json", map[string]string{"name": folder}); err != nil {
			t.Fatal(err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.Base(folder), Filename)); err != nil {
			t.Fatalf("database for %q is not in that folder: %v", folder, err)
		}
		reopened, err := Open(folder, false, Config{Documents: []string{"settings.json"}})
		if err != nil {
			t.Fatal(err)
		}
		var saved map[string]string
		if found, err := reopened.LoadJSON("settings.json", &saved); err != nil || !found || saved["name"] != folder {
			t.Fatalf("reopened %q = %v %v %v", folder, saved, found, err)
		}
		_ = reopened.Close()
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 2 {
		t.Fatalf("unexpected files beside the configured folders: %v", entries)
	}
}
