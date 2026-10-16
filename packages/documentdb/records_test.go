package documentdb

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func openEntries(t *testing.T, root string, config Config, name string) *Store {
	t.Helper()
	store, err := Open(root, true, config)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnableEntries(name); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestEntryDocumentsGrowBeyondWholeDocumentLimit(t *testing.T) {
	root := t.TempDir()
	config := Config{Documents: []string{"metadata.json"}}
	store := openEntries(t, root, config, "metadata.json")
	record := strings.Repeat("r", EntrySizeLimit-64)
	want := make(map[string]string)
	for index := range DocumentSizeLimit/len(record) + 2 {
		key := "item-" + strconv.Itoa(index)
		want[key] = record
		if err := store.SaveEntry("metadata.json", key, record); err != nil {
			t.Fatalf("entry %d: %v", index, err)
		}
	}
	if err := store.SaveEntry("metadata.json", "oversized", strings.Repeat("x", EntrySizeLimit)); err == nil {
		t.Fatal("entry above the per-entry limit accepted")
	}
	exported, err := store.Export()
	if err != nil || len(exported["metadata.json"]) <= DocumentSizeLimit {
		t.Fatalf("export = %d bytes, %v", len(exported["metadata.json"]), err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(root, false, config)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var got map[string]string
	if found, err := reopened.LoadJSON("metadata.json", &got); err != nil || !found || !reflect.DeepEqual(got, want) {
		t.Fatalf("reopened %d records, found=%t, %v", len(got), found, err)
	}
}

func TestSaveEntriesCommitsOnlyRequestedKeysAtomically(t *testing.T) {
	store := openEntries(t, t.TempDir(), Config{Documents: []string{"progress.json", "settings.json"}}, "progress.json")
	defer store.Close()
	if err := store.SaveEntries("progress.json", map[string]any{"one": 1, "two": 2, "three": 3}, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveEntries("progress.json", map[string]any{"two": 20, "four": 4}, []string{"three", "missing"}); err != nil {
		t.Fatal(err)
	}
	rejected := []struct {
		name    string
		set     map[string]any
		deleted []string
	}{
		{"progress.json", map[string]any{"one": 100, "": 1}, nil},
		{"progress.json", map[string]any{"one": 100}, []string{"one"}},
		{"progress.json", map[string]any{"one": 100, "bad": func() {}}, nil},
		{"progress.json", map[string]any{"one": 100}, []string{strings.Repeat("k", 257)}},
		{"settings.json", map[string]any{"one": 100}, nil},
		{"settings.json", nil, []string{"one"}},
		{"unknown.json", map[string]any{"one": 100}, nil},
	}
	for _, test := range rejected {
		if err := store.SaveEntries(test.name, test.set, test.deleted); err == nil {
			t.Fatalf("invalid batch accepted: %s %#v %#v", test.name, test.set, test.deleted)
		}
	}
	err := store.Commit(t.Context(), Batch{Documents: map[string]any{"settings.json": map[string]int{"revision": 1}}, Entries: map[string]EntryChanges{"progress.json": {Set: map[string]any{"one": 10}, Delete: []string{"four"}}}})
	if err != nil {
		t.Fatal(err)
	}
	var progress map[string]int
	if _, err := store.LoadJSON("progress.json", &progress); err != nil || !reflect.DeepEqual(progress, map[string]int{"one": 10, "two": 20}) {
		t.Fatalf("progress = %#v, %v", progress, err)
	}
	var settings map[string]int
	if _, err := store.LoadJSON("settings.json", &settings); err != nil || settings["revision"] != 1 {
		t.Fatalf("settings = %#v, %v", settings, err)
	}
	if err := store.Commit(t.Context(), Batch{Documents: map[string]any{"progress.json": map[string]int{}}, Entries: map[string]EntryChanges{"progress.json": {Set: map[string]any{"one": 1}}}}); err == nil {
		t.Fatal("document and entry changes for one name accepted")
	}
	if _, err := store.Export(); err != nil {
		t.Fatalf("rejected batches left invalid counters: %v", err)
	}
}

func TestSchemaTwoDatabaseUpgradesInPlace(t *testing.T) {
	root := t.TempDir()
	raw := openRaw(t, filepath.Join(root, Filename))
	schema := `CREATE TABLE state(name TEXT PRIMARY KEY NOT NULL, value BLOB NOT NULL CHECK(length(value)<=16777216)) STRICT;` + entrySchemaVersionTwo
	if _, err := raw.ExecContext(t.Context(), schema); err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`INSERT INTO state(name,value,entry_mode,entry_bytes,entry_count) VALUES('progress.json',CAST('{}' AS BLOB),1,` + strconv.Itoa(1+len(`"viewer:a"`)+len(`10`)+2) + `,1)`,
		`INSERT INTO state_entries(name,key,value,cost) VALUES('progress.json','viewer:a',CAST('10' AS BLOB),` + strconv.Itoa(len(`"viewer:a"`)+len(`10`)+2) + `)`,
		`INSERT INTO state(name,value) VALUES('settings.json',CAST('{"name":"Server"}' AS BLOB))`,
	}
	for _, statement := range statements {
		if _, err := raw.ExecContext(t.Context(), statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	config := Config{Documents: []string{"progress.json", "settings.json"}, ValidateBeforeMigration: true}
	store, err := Open(root, true, config)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SaveEntry("progress.json", "viewer:b", 20); err != nil {
		t.Fatal(err)
	}
	exported, err := store.Export()
	if err != nil {
		t.Fatal(err)
	}
	var progress map[string]int
	if err := json.Unmarshal(exported["progress.json"], &progress); err != nil || !reflect.DeepEqual(progress, map[string]int{"viewer:a": 10, "viewer:b": 20}) || string(exported["settings.json"]) != `{"name":"Server"}` {
		t.Fatalf("upgraded export = %q, %v", exported, err)
	}
	var version int
	if err := store.db.QueryRowContext(context.Background(), `PRAGMA user_version`).Scan(&version); err != nil || version != SchemaVersion || SchemaVersion != 3 {
		t.Fatalf("schema version = %d, %v", version, err)
	}
}

const entrySchemaVersionTwo = `
ALTER TABLE state ADD COLUMN entry_mode INTEGER NOT NULL DEFAULT 0 CHECK(entry_mode IN (0,1));
ALTER TABLE state ADD COLUMN entry_bytes INTEGER NOT NULL DEFAULT 1 CHECK(entry_bytes BETWEEN 1 AND 16777216);
ALTER TABLE state ADD COLUMN entry_count INTEGER NOT NULL DEFAULT 0 CHECK(entry_count BETWEEN 0 AND 1000000);
CREATE TABLE state_entries (
    name TEXT NOT NULL, key TEXT NOT NULL CHECK(length(CAST(key AS BLOB)) BETWEEN 1 AND 256),
    value BLOB NOT NULL CHECK(length(value) <= 16777216),
    cost INTEGER NOT NULL CHECK(cost BETWEEN 1 AND 16777216),
    PRIMARY KEY(name,key)
) STRICT, WITHOUT ROWID;
PRAGMA user_version=2;`
