package server

import (
	"strconv"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/database"
)

func reopenMetadataStore(t *testing.T, data string) *metadataStore {
	t.Helper()
	stateDB, err := database.Open(data, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stateDB.Close() })
	store := newMetadataStore(MetadataConfig{}, data, t.TempDir(), stateDB)
	if store.err != nil {
		t.Fatal(store.err)
	}
	return store
}

func TestMetadataRecordsPersistBeyondTheWholeDocumentLimit(t *testing.T) {
	data := t.TempDir()
	stateDB, err := database.Open(data, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := stateDB.SaveJSON("metadata.json", map[string]metadataRecord{"legacy": {Title: "Legacy", Year: "2001"}}); err != nil {
		t.Fatal(err)
	}
	if err := stateDB.Close(); err != nil {
		t.Fatal(err)
	}
	store := reopenMetadataStore(t, data)
	records := make(map[string]metadataRecord)
	for index := range 6_000 {
		record := benchmarkMetadataRecord(index)
		record.Plot = strings.Repeat("Episode plot. ", 200)
		records["episode"+strconv.Itoa(index)] = record
	}
	if err := store.setAll(records); err != nil {
		t.Fatalf("records above 16 MiB were rejected: %v", err)
	}
	edited := benchmarkMetadataRecord(7)
	edited.Title = "Edited"
	if err := store.set("episode7", edited); err != nil {
		t.Fatal(err)
	}
	reloaded := reopenMetadataStore(t, data)
	if len(reloaded.records) != 6_001 || reloaded.records["legacy"].Title != "Legacy" || reloaded.records["episode7"].Title != "Edited" || reloaded.records["episode5999"].Plot != records["episode5999"].Plot {
		t.Fatalf("reloaded %d records, legacy=%q edited=%q", len(reloaded.records), reloaded.records["legacy"].Title, reloaded.records["episode7"].Title)
	}
}

func TestMetadataRecordLimitRejectsWithoutSideEffects(t *testing.T) {
	data := t.TempDir()
	store := reopenMetadataStore(t, data)
	if err := store.set("kept", metadataRecord{Title: "Kept"}); err != nil {
		t.Fatal(err)
	}
	overflow := make(map[string]metadataRecord, metadataRecordLimit)
	for index := range metadataRecordLimit {
		overflow["item"+strconv.Itoa(index)] = metadataRecord{Title: "T"}
	}
	if err := store.setAll(overflow); err == nil {
		t.Fatal("record limit overflow accepted")
	}
	invalid := map[string]metadataRecord{"bad/id": {Title: "Bad"}, "other": {Title: "Other"}}
	if err := store.setAll(invalid); err == nil {
		t.Fatal("invalid record id accepted")
	}
	if len(store.records) != 1 {
		t.Fatalf("rejected writes changed memory: %d records", len(store.records))
	}
	if reloaded := reopenMetadataStore(t, data); len(reloaded.records) != 1 || reloaded.records["kept"].Title != "Kept" {
		t.Fatalf("rejected writes changed storage: %#v", reloaded.records)
	}
}
