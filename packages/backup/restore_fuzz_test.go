package backup_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func FuzzRestoreWritesOnlyConfigurationState(f *testing.F) { //nolint:cyclop,gocognit // One property checks every filesystem effect of the public restore boundary.
	f.Add([]byte("not a backup"))
	f.Add(servertest.TarGzipFile(f, "settings.json", `{}`))
	f.Add(servertest.TarGzipFile(f, "../escaped.json", `{}`))
	f.Fuzz(func(t *testing.T, archive []byte) {
		if len(archive) > 1<<20 {
			return
		}
		destination, outside, keep := prepareFuzzRestore(t)
		_ = testArchive.Restore(bytes.NewReader(archive), destination)
		assertFuzzRestoreConfined(t, destination, outside, keep)
	})
}

func prepareFuzzRestore(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	destination := filepath.Join(root, "restore")
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	outside, keep := filepath.Join(root, "outside"), filepath.Join(destination, "keep")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	return destination, outside, keep
}

func assertFuzzRestoreConfined(t *testing.T, destination, outside, keep string) {
	t.Helper()
	assertFuzzFileUnchanged(t, outside, "outside", "restore escaped destination")
	assertFuzzFileUnchanged(t, keep, "keep", "restore changed unmanaged state")
	entries, err := os.ReadDir(destination)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"api_keys.json": true, "audit.jsonl": true, "collections.json": true, "configuration.json": true,
		"history.json": true, "lists.json": true, "metadata.json": true,
		"playback_markers.json": true, "playlists.json": true, "profiles.json": true, "progress.json": true,
		"server-identity.json": true, "sessions.json": true, "settings.json": true,
	}
	for _, entry := range entries {
		assertFuzzRestoreEntry(t, entry, allowed)
	}
}

func assertFuzzFileUnchanged(t *testing.T, path, want, message string) {
	t.Helper()
	if data, err := os.ReadFile(path); err != nil || string(data) != want {
		t.Fatalf("%s: %q, %v", message, data, err)
	}
}

func assertFuzzRestoreEntry(t *testing.T, entry os.DirEntry, allowed map[string]bool) {
	t.Helper()
	if entry.Name() != "keep" && !allowed[entry.Name()] {
		t.Fatalf("restore created unexpected entry %q", entry.Name())
	}
	info, err := entry.Info()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("restore created unsafe entry %q: %v, %v", entry.Name(), info, err)
	}
}
