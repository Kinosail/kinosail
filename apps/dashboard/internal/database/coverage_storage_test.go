package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func coverageStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(t.Context(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestCoverageDatabaseOpenBoundaries(t *testing.T) {
	if _, err := Open(nil, t.TempDir()); err == nil { //nolint:staticcheck // Verify the public nil-context rejection contract.
		t.Fatal("nil context accepted")
	}
	if _, err := Open(t.Context(), " \t\r\n"); err == nil {
		t.Fatal("empty path accepted")
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(t.Context(), file); err == nil {
		t.Fatal("file used as directory")
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(t.TempDir(), link); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(t.Context(), link); err == nil {
		t.Fatal("symlink directory accepted")
	}
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, Filename), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(t.Context(), directory); err == nil {
		t.Fatal("directory used as database")
	}
	if err := (*Store)(nil).Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCoverageDatabaseRejectsInvalidDocuments(t *testing.T) {
	store := coverageStore(t)
	for _, values := range []map[string]any{nil, {"unknown": true}, {"board.json": make(chan int)}, {"board.json": strings.Repeat("x", DocumentSizeLimit)}} {
		if err := store.SaveJSONBatch(t.Context(), values); err == nil {
			t.Fatal("invalid batch accepted")
		}
	}
	if err := store.SaveJSONBatch(nil, map[string]any{"board.json": true}); err == nil { //nolint:staticcheck // Verify nil-context writes are rejected before persistence.
		t.Fatal("nil context accepted")
	}
	var target any
	if _, err := store.LoadJSON(nil, "board.json", &target); err == nil { //nolint:staticcheck // Verify nil-context reads are rejected.
		t.Fatal("nil load context accepted")
	}
	if _, err := store.LoadJSON(t.Context(), "unknown", &target); err == nil {
		t.Fatal("unknown document loaded")
	}
	if found, err := store.LoadJSON(t.Context(), "board.json", &target); err != nil || found {
		t.Fatalf("rejected writes persisted = %v, %v", found, err)
	}
}

func TestCoverageDatabaseReadFailures(t *testing.T) {
	store := coverageStore(t)
	for _, test := range []struct {
		value  string
		target any
	}{
		{"{", new(any)}, {`{"title":"Home"}`, new(int)},
	} {
		if _, err := store.db.ExecContext(t.Context(), `INSERT OR REPLACE INTO state(name,value) VALUES('board.json',?)`, []byte(test.value)); err != nil {
			t.Fatal(err)
		}
		if found, err := store.LoadJSON(t.Context(), "board.json", test.target); err == nil || found {
			t.Fatalf("invalid document loaded = %v, %v", found, err)
		}
	}
	if err := store.db.Close(); err != nil {
		t.Fatal(err)
	}
	var target any
	if _, err := store.LoadJSON(t.Context(), "board.json", &target); err == nil {
		t.Fatal("closed database read succeeded")
	}
	if err := store.SaveJSON(t.Context(), "board.json", true); err == nil {
		t.Fatal("closed database write succeeded")
	}
	if err := store.initialize(t.Context()); err == nil {
		t.Fatal("closed database initialized")
	}
}

func TestCoverageDatabaseSchemaRejection(t *testing.T) {
	for _, schema := range []string{"PRAGMA user_version=2", "PRAGMA user_version=0"} {
		store := coverageStore(t)
		if _, err := store.db.ExecContext(t.Context(), schema); err != nil {
			t.Fatal(err)
		}
		if err := store.initialize(t.Context()); err == nil {
			t.Fatalf("invalid schema accepted: %s", schema)
		}
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, Filename), []byte("not SQLite"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(t.Context(), directory); err == nil {
		t.Fatal("invalid SQLite file accepted")
	}
}
