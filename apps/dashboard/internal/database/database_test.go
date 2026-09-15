package database

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveJSONBatchRollsBackEveryDocumentOnWriteFailure(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	original := map[string]string{"title": "Original"}
	if err := store.SaveJSON(ctx, "board.json", original); err != nil {
		t.Fatalf("seed board: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `CREATE TRIGGER fail_sessions
		BEFORE INSERT ON state WHEN NEW.name = 'sessions.json'
		BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	err = store.SaveJSONBatch(ctx, map[string]any{
		"board.json":    map[string]string{"title": "Changed"},
		"sessions.json": map[string]any{"sessions": []any{}},
	})
	if err == nil {
		t.Fatal("SaveJSONBatch error = nil, want injected transaction failure")
	}
	var board map[string]string
	found, err := store.LoadJSON(ctx, "board.json", &board)
	if err != nil {
		t.Fatalf("load board: %v", err)
	}
	if !found || board["title"] != "Original" {
		t.Fatalf("board after failed batch = %v, want original value", board)
	}
	var sessions any
	found, err = store.LoadJSON(ctx, "sessions.json", &sessions)
	if err != nil {
		t.Fatalf("load sessions: %v", err)
	}
	if found {
		t.Fatalf("sessions document was committed after failed batch: %v", sessions)
	}
}

func TestOpenEnforcesPrivateDataPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX permission bits")
	}
	root := t.TempDir()
	dataDir := filepath.Join(root, "dashboard-data")
	if err := os.Mkdir(dataDir, 0o755); err != nil { // #nosec G301 -- Verify Open repairs an existing directory with excessive permissions.
		t.Fatalf("create data directory: %v", err)
	}
	store, err := Open(context.Background(), dataDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	assertPermissions(t, dataDir, 0o700)
	assertPermissions(t, store.Path(), 0o600)
	for _, suffix := range []string{"-wal", "-shm"} {
		path := store.Path() + suffix
		if _, err := os.Stat(path); err == nil {
			assertPermissions(t, path, 0o600)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", path, err)
		}
	}
}

func TestDataDirectoryHasOneProcessOwner(t *testing.T) {
	directory := t.TempDir()
	first, err := Open(context.Background(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := Open(context.Background(), directory); err == nil {
		_ = second.Close()
		t.Fatal("second state owner opened the same data directory")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(context.Background(), directory)
	if err != nil {
		t.Fatalf("released data directory did not reopen: %v", err)
	}
	defer reopened.Close()
	if runtime.GOOS != "windows" {
		assertPermissions(t, filepath.Join(directory, ".kinosail-dashboard.lock"), 0o600)
	}
}

func assertPermissions(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("permissions for %s = %#o, want %#o", path, got, want)
	}
}
