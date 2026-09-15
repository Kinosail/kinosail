package servertest

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sharedbackup "github.com/MikeO7/kinosail/packages/backup"
)

// SQLiteDatabase is the durable document boundary exercised by an app backup adapter.
type SQLiteDatabase interface {
	SaveJSON(string, any) error
	LoadJSON(string, any) (bool, error)
	Close() error
}

// SQLiteBackupFixture binds one app's database and archive service.
type SQLiteBackupFixture[Database SQLiteDatabase] struct {
	Filename string
	Open     func(string, bool) (Database, error)
	Write    func(io.Writer, string) error
	Restore  func(io.Reader, string) error
}

// SQLiteBackupContract verifies logical database export and safe replacement behavior.
func SQLiteBackupContract[Database SQLiteDatabase](t *testing.T, fixture SQLiteBackupFixture[Database]) {
	t.Helper()
	t.Run("SQLite state round trips through logical backup", fixture.sqliteRoundTrip)
	t.Run("non-regular SQLite paths are rejected", fixture.rejectNonRegularSQLite)
	t.Run("restore rejects a non-regular SQLite target before changing state", fixture.rejectNonRegularRestoreTarget)
}

func (fixture SQLiteBackupFixture[Database]) sqliteRoundTrip(t *testing.T) { //nolint:cyclop // One end-to-end contract verifies every restored document below the project ceiling.
	source := t.TempDir()
	store, err := fixture.Open(source, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveJSON("settings.json", map[string]any{"name": "SQLite Home", "libraries": []string{"."}, "subtitleLanguages": []string{"en"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveJSON("smart_playlists.json", map[string]any{"owner:Recent": map[string]string{"sort": "added"}}); err != nil {
		t.Fatal(err)
	}
	largeMetadata := strings.Repeat("m", 3<<20)
	if err := store.SaveJSON("metadata.json", map[string]string{"catalog": largeMetadata}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	if err := fixture.Write(&archive, source); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	stale, err := fixture.Open(destination, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := stale.SaveJSON("settings.json", map[string]string{"name": "Stale"}); err != nil {
		t.Fatal(err)
	}
	if err := stale.Close(); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Restore(bytes.NewReader(archive.Bytes()), destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, fixture.Filename)); !os.IsNotExist(err) {
		t.Fatalf("restore retained stale database: %v", err)
	}
	fixture.assertRestoredDocuments(t, destination, largeMetadata)
}

func (fixture SQLiteBackupFixture[Database]) assertRestoredDocuments(t *testing.T, destination, largeMetadata string) { //nolint:cyclop // One assertion covers the complete logical document set below the project ceiling.
	t.Helper()
	restored, err := fixture.Open(destination, false)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var settings map[string]any
	if found, err := restored.LoadJSON("settings.json", &settings); err != nil || !found || settings["name"] != "SQLite Home" {
		t.Fatalf("restored settings = %#v, %v, %v", settings, found, err)
	}
	var playlists map[string]any
	if found, err := restored.LoadJSON("smart_playlists.json", &playlists); err != nil || !found || playlists["owner:Recent"] == nil {
		t.Fatalf("restored playlists = %#v, %v, %v", playlists, found, err)
	}
	var metadata map[string]string
	if found, err := restored.LoadJSON("metadata.json", &metadata); err != nil || !found || metadata["catalog"] != largeMetadata {
		t.Fatalf("large metadata restored = %d bytes, %v, %v", len(metadata["catalog"]), found, err)
	}
}

func (fixture SQLiteBackupFixture[Database]) rejectNonRegularSQLite(t *testing.T) {
	for name, prepare := range map[string]func(string) error{
		"directory": func(path string) error { return os.Mkdir(path, 0o700) },
		"symlink": func(path string) error {
			target := filepath.Join(filepath.Dir(path), "outside.db")
			if err := os.WriteFile(target, []byte("outside"), 0o600); err != nil {
				return err
			}
			return os.Symlink(target, path)
		},
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			if err := prepare(filepath.Join(directory, fixture.Filename)); err != nil {
				t.Fatal(err)
			}
			if err := fixture.Write(io.Discard, directory); err == nil {
				t.Fatal("non-regular SQLite path was backed up")
			}
		})
	}
}

func (fixture SQLiteBackupFixture[Database]) rejectNonRegularRestoreTarget(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "settings.json"), []byte(`{"name":"After","subtitleLanguages":["en"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	if err := fixture.Write(&archive, source); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	settings := filepath.Join(destination, "settings.json")
	if err := os.WriteFile(settings, []byte(`{"name":"Before"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(destination, fixture.Filename), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Restore(bytes.NewReader(archive.Bytes()), destination); err == nil {
		t.Fatal("restore over a non-regular database succeeded")
	}
	if data, err := os.ReadFile(settings); err != nil || string(data) != `{"name":"Before"}` {
		t.Fatalf("failed restore changed settings: %q, %v", data, err)
	}
}

// BackupAdapterContract verifies encrypted adapters and the version-bound command.
func BackupAdapterContract(t *testing.T, writeEncrypted func(io.Writer, string, string) error, verify func(io.Reader, string) error, command sharedbackup.Command) {
	t.Helper()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "settings.json"), []byte(`{"name":"Home","libraries":["."],"subtitleLanguages":["en"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	if err := writeEncrypted(&archive, directory, "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if err := verify(bytes.NewReader(archive.Bytes()), "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if err := verify(bytes.NewReader(archive.Bytes()), "wrong passphrase"); err == nil {
		t.Fatal("encrypted backup accepted the wrong passphrase")
	}
	var plain bytes.Buffer
	if handled, err := command([]string{"backup"}, nil, &plain, directory, ""); err != nil || !handled || plain.Len() == 0 {
		t.Fatalf("backup command = handled %t, bytes %d, error %v", handled, plain.Len(), err)
	}
}
