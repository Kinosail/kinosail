package documentdb

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSchemaOneProgressUpgradeAndReopen(t *testing.T) { //nolint:cyclop // Migration, record update, and reopen form one persistence regression.
	root := t.TempDir()
	raw := openRaw(t, filepath.Join(root, Filename))
	if _, err := raw.ExecContext(t.Context(), `CREATE TABLE state(name TEXT PRIMARY KEY NOT NULL, value BLOB NOT NULL CHECK(length(value)<=16777216)) STRICT; PRAGMA user_version=1;`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(t.Context(), `INSERT INTO state(name,value) VALUES('progress.json',?)`, []byte(`{"viewer:item":10}`)); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	config := Config{Documents: []string{"progress.json"}, ValidateBeforeMigration: true}
	store, err := Open(root, true, config)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnableEntries("progress.json"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveEntry("progress.json", "viewer:second", 20); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(root, true, config)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var got map[string]int
	if _, err := store.LoadJSON("progress.json", &got); err != nil || got["viewer:item"] != 10 || got["viewer:second"] != 20 || len(got) != 2 {
		t.Fatalf("upgraded state = %#v, %v", got, err)
	}
}

func TestEntryValidationPrecedesRetiredCleanup(t *testing.T) { //nolint:cyclop // Corrupt imports must preserve both active and retired durable rows.
	root := t.TempDir()
	config := Config{Documents: []string{"progress.json"}, RetiredDocuments: []string{"retired.json"}, ValidateBeforeMigration: true}
	store, err := Open(root, true, config)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnableEntries("progress.json"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveEntry("progress.json", "viewer:item", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(t.Context(), `INSERT INTO state(name,value) VALUES('retired.json',?)`, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(t.Context(), `UPDATE state SET entry_count=0 WHERE name='progress.json'`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if reopened, err := Open(root, true, config); err == nil || reopened != nil {
		t.Fatalf("invalid state accepted: %v", err)
	}
	raw := openRaw(t, filepath.Join(root, Filename))
	defer raw.Close()
	var count int
	if err := raw.QueryRowContext(t.Context(), `SELECT count(*) FROM state WHERE name='retired.json'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("rejection changed retired state: %d, %v", count, err)
	}
}

func TestAmbiguousEntryMigrationHasNoSideEffects(t *testing.T) {
	for _, data := range [][]byte{[]byte(`{"item":1,"item":2}`), []byte(`{"item":1,"\u0069tem":2}`), []byte(`{"item":{"position":1,"position":2}}`), []byte("{\"\xff\":1}")} {
		store, err := Open(t.TempDir(), true, Config{Documents: []string{"progress.json"}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.db.ExecContext(t.Context(), `INSERT INTO state(name,value) VALUES('progress.json',?)`, data); err != nil {
			t.Fatal(err)
		}
		if err := store.EnableEntries("progress.json"); err == nil {
			t.Fatal("ambiguous migration accepted")
		}
		var retained []byte
		var mode, count int
		if err := store.db.QueryRowContext(t.Context(), `SELECT value,entry_mode,(SELECT count(*) FROM state_entries) FROM state WHERE name='progress.json'`).Scan(&retained, &mode, &count); err != nil || string(retained) != string(data) || mode != 0 || count != 0 {
			t.Fatalf("rejected migration changed state: %v", err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEntriesPreserveWholeDocumentAndBackupContract(t *testing.T) { //nolint:cyclop,gocognit // Record updates, document replacement, and backup restore share one fixture.
	root := t.TempDir()
	config := Config{Documents: []string{"progress.json", "settings.json"}}
	store, err := Open(root, true, config)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveJSON("progress.json", map[string]int{"viewer:one": 10, "viewer:two": 20}); err != nil {
		t.Fatal(err)
	}
	if err := store.EnableEntries("progress.json"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveEntry("progress.json", "viewer:one", 30); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"viewer:one": 30, "viewer:two": 20}
	var got map[string]int
	if found, err := store.LoadJSON("progress.json", &got); err != nil || !found || !reflect.DeepEqual(got, want) {
		t.Fatalf("load = %#v, %v", got, err)
	}
	exported, err := store.Export()
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(exported["progress.json"], &got); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("export = %#v, %v", got, err)
	}
	replacement := map[string]int{"viewer:three": 40}
	if err := store.SaveJSONBatch(map[string]any{"progress.json": replacement, "settings.json": map[string]bool{"enabled": true}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(root, true, config)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	got = nil
	if found, err := store.LoadJSON("progress.json", &got); err != nil || !found || !reflect.DeepEqual(got, replacement) {
		t.Fatalf("reopen = %#v, %v", got, err)
	}
	var count int
	if err := store.db.QueryRowContext(t.Context(), `SELECT count(*) FROM state_entries`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("obsolete rows = %d, %v", count, err)
	}
}

func TestRejectedEntriesDoNotChangeDurableState(t *testing.T) { //nolint:cyclop // The rejection table checks the same baseline document after every attempt.
	store, err := Open(t.TempDir(), true, Config{Documents: []string{"progress.json"}})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.EnableEntries("progress.json"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveEntry("progress.json", "viewer:item", 10); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"", strings.Repeat("x", 257), string([]byte{0xff})} {
		if err := store.SaveEntry("progress.json", key, 20); err == nil {
			t.Fatal("invalid key accepted")
		}
	}
	if err := store.SaveEntry("unknown.json", "viewer:item", 20); err == nil {
		t.Fatal("unknown document accepted")
	}
	if err := store.SaveEntry("progress.json", "other", strings.Repeat("x", DocumentSizeLimit)); err == nil {
		t.Fatal("oversized record accepted")
	}
	if err := store.SaveJSONBatch(map[string]any{"progress.json": map[string]int{"replacement": 1}, "unknown.json": true}); err == nil {
		t.Fatal("invalid batch accepted")
	}
	var got map[string]int
	if _, err := store.LoadJSON("progress.json", &got); err != nil || len(got) != 1 || got["viewer:item"] != 10 {
		t.Fatalf("rejected write changed state: %#v, %v", got, err)
	}
}

func TestEntryAggregateLimitRollsBackBothTables(t *testing.T) {
	store, err := Open(t.TempDir(), true, Config{Documents: []string{"progress.json"}})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.EnableEntries("progress.json"); err != nil {
		t.Fatal(err)
	}
	large := strings.Repeat("x", DocumentSizeLimit/2)
	if err := store.SaveEntry("progress.json", "first", large); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveEntry("progress.json", "second", large); err == nil {
		t.Fatal("aggregate overflow accepted")
	}
	var got map[string]string
	if _, err := store.LoadJSON("progress.json", &got); err != nil || len(got) != 1 || got["first"] != large {
		t.Fatalf("rollback failed: %d records, %v", len(got), err)
	}
	if _, err := store.Export(); err != nil {
		t.Fatalf("rollback left invalid counters: %v", err)
	}
}

func TestEntryMigrationAcceptsEmptyLegacyProgress(t *testing.T) {
	store, err := Open(t.TempDir(), true, Config{Documents: []string{"progress.json"}})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SaveJSON("progress.json", nil); err != nil {
		t.Fatal(err)
	}
	if err := store.EnableEntries("progress.json"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveEntry("progress.json", "viewer:item", 1); err != nil {
		t.Fatal(err)
	}
}
