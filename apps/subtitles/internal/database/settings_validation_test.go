package database

import (
	"database/sql"
	"net/url"
	"path/filepath"
	"testing"
)

func TestOpenRejectsSemanticInvalidDatabaseBeforeInitializationWrites(t *testing.T) { //nolint:cyclop // Every invalid persisted-settings boundary is checked without side effects.
	directory := t.TempDir()
	store, err := Open(directory, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SaveJSON("settings.json", map[string]any{"subtitleLanguages": []string{"en"}}); err != nil {
		t.Fatal(err)
	}
	raw := openRawDatabase(t, directory)
	if _, err = raw.ExecContext(t.Context(), `UPDATE state SET value = ? WHERE name = 'settings.json'`, []byte(`{"subtitleLanguages":[]}`)); err != nil {
		t.Fatal(err)
	}
	if _, err = raw.ExecContext(t.Context(), `INSERT INTO state(name, value) VALUES('live-tv.json', ?)`, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if err = raw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	if reopened, err := Open(directory, false); err == nil || reopened != nil {
		t.Fatalf("Open() = %#v, %v", reopened, err)
	}
	raw, err = sql.Open("sqlite3", (&url.URL{Scheme: "file", Path: filepath.Join(directory, Filename)}).String())
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	var retained int
	if err = raw.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM state WHERE name = 'live-tv.json'`).Scan(&retained); err != nil || retained != 1 {
		t.Fatalf("rejected open initialized state: retained=%d error=%v", retained, err)
	}
}

func TestSaveRejectsInvalidSupporterGrantWithoutReplacingSettings(t *testing.T) {
	store, err := Open(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	valid := map[string]any{"name": "Home", "supporter": map[string]any{"patronOrder": map[string]string{"certificate": "certificate"}}}
	if err = store.SaveJSON("settings.json", valid); err != nil {
		t.Fatal(err)
	}
	invalid := map[string]any{"name": "Changed", "supporter": map[string]any{"livingStandard": map[string]any{"unexpected": true}}}
	if err = store.SaveJSON("settings.json", invalid); err == nil {
		t.Fatal("invalid supporter grant was saved")
	}
	var retained map[string]any
	if found, loadErr := store.LoadJSON("settings.json", &retained); loadErr != nil || !found || retained["name"] != "Home" {
		t.Fatalf("rejected save changed settings: %#v, %v, %v", retained, found, loadErr)
	}
}

func openRawDatabase(t *testing.T, directory string) *sql.DB {
	t.Helper()
	raw, err := sql.Open("sqlite3", (&url.URL{Scheme: "file", Path: filepath.Join(directory, Filename)}).String())
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
