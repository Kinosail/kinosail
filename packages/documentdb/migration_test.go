package documentdb

import (
	"path/filepath"
	"testing"
)

func TestOpenRemovesRetiredDocuments(t *testing.T) {
	directory := t.TempDir()
	store, err := Open(directory, false, testConfig)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	raw := openRaw(t, filepath.Join(directory, Filename))
	if _, err := raw.ExecContext(t.Context(), `INSERT INTO state(name, value) VALUES('live-tv.json', ?)`, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()
	store, err = Open(directory, false, testConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if exported, err := store.Export(); err != nil || len(exported) != 0 {
		t.Fatalf("retired state remained: %#v, %v", exported, err)
	}
}
