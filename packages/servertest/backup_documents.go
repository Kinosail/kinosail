package servertest

import (
	"bytes"
	"errors"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const largeEntryDocumentBytes = 17 << 20

func (fixture SQLiteBackupFixture[Database]) everyDocumentRoundTrips(t *testing.T) {
	source := t.TempDir()
	store, err := fixture.Open(source, false)
	if err != nil {
		t.Fatal(err)
	}
	want := make(map[string]any, len(fixture.Documents))
	for _, name := range fixture.Documents {
		want[name] = fixture.populateDocument(t, store, name)
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
	if err := errors.Join(stale.SaveJSON("collections.json", map[string]string{"stale": "state"}), stale.Close()); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Restore(bytes.NewReader(archive.Bytes()), destination); err != nil {
		t.Fatal(err)
	}
	restored, err := fixture.Open(destination, false)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	for _, name := range fixture.Documents {
		var got map[string]any
		if found, err := restored.LoadJSON(name, &got); err != nil || !found || !reflect.DeepEqual(got, want[name]) {
			t.Fatalf("restored %s: found=%t err=%v equal=%t", name, found, err, reflect.DeepEqual(got, want[name]))
		}
		if slices.Contains(fixture.EntryDocuments, name) {
			if err := restored.SaveEntry(name, "after-restore", map[string]string{"entry": name}); err != nil {
				t.Fatalf("restored %s is not in entry storage: %v", name, err)
			}
		}
	}
}

func (fixture SQLiteBackupFixture[Database]) populateDocument(t *testing.T, store Database, name string) any {
	t.Helper()
	if name == "settings.json" {
		settings := map[string]any{"name": "Every Document", "libraries": []any{"."}, "subtitleLanguages": []any{"en"}}
		if err := store.SaveJSON(name, settings); err != nil {
			t.Fatal(err)
		}
		return settings
	}
	if !slices.Contains(fixture.EntryDocuments, name) {
		document := map[string]any{"document": name}
		if err := store.SaveJSON(name, document); err != nil {
			t.Fatalf("save %s: %v", name, err)
		}
		return document
	}
	if err := store.EnableEntries(name); err != nil {
		t.Fatalf("enable entries for %s: %v", name, err)
	}
	document := make(map[string]any)
	value := map[string]any{"value": strings.Repeat("e", 1000<<10)}
	for index := 0; index*len(value["value"].(string)) <= largeEntryDocumentBytes; index++ {
		document[name+"-"+strconv.Itoa(index)] = value
	}
	if err := store.SaveEntries(name, document, nil); err != nil {
		t.Fatalf("save entries for %s: %v", name, err)
	}
	return document
}
