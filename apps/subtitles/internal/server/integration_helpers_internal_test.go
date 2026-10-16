package server

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	sharedmetadata "github.com/MikeO7/kinosail/packages/metadata"
)

func TestAtomicFileWriterIsSafeAcrossConcurrentCalls(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "nested", "cache.bin")
	var wait sync.WaitGroup
	for index := range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := writeAtomicFile(target, []byte(strings.Repeat("abcdefgh"[index:index+1], 1024))); err != nil {
				t.Error(err)
			}
		}()
	}
	wait.Wait()
	data, err := os.ReadFile(target)
	if err != nil || len(data) != 1024 || strings.Trim(string(data), string(data[0])) != "" {
		t.Fatalf("atomic cache file = %d bytes, %v", len(data), err)
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomicFile(filepath.Join(blocked, "child"), []byte("data")); err == nil {
		t.Fatal("cache write below a file succeeded")
	}
}

func TestAtomicJSONWriterIsSafeAcrossConcurrentCalls(t *testing.T) {
	target := filepath.Join(t.TempDir(), "state.json")
	var wait sync.WaitGroup
	for index := range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := saveJSON(target, map[string]int{"index": index}); err != nil {
				t.Error(err)
			}
		}()
	}
	wait.Wait()
	jsonData, err := os.ReadFile(target)
	var state map[string]int
	if err != nil || decodeExternalJSON(strings.NewReader(string(jsonData)), 1024, &state) != nil || state["index"] < 0 || state["index"] > 7 {
		t.Fatalf("atomic JSON state = %q, %#v, %v", jsonData, state, err)
	}
}

func TestMetadataStateAndFormValidation(t *testing.T) {
	t.Parallel()
	record, ok := sharedmetadata.RecordFromForm(url.Values{
		"title": {" Arrival "}, "year": {"2016"}, "plot": {" Plot "}, "rating": {"PG-13"}, "tagline": {" Tagline "}, "genres": {"Drama"},
	}, metadataRecord{})
	if !ok || record.Title != "Arrival" || !record.Owner || !sharedmetadata.Bounded(record) {
		t.Fatalf("valid metadata form = %#v, %v", record, ok)
	}
	for _, invalid := range []metadataRecord{
		{Title: "Movie", Year: "year"},
		{Title: "Movie", ShowYear: "20x6"},
		{Title: "Movie", ProviderIDs: map[string]string{"": "value"}},
		{Title: "Movie", ShowProviderIDs: map[string]string{"tmdb": ""}},
	} {
		if sharedmetadata.Bounded(invalid) {
			t.Fatalf("invalid metadata was accepted: %#v", invalid)
		}
	}
	if sharedmetadata.ValidateRecords(map[string]metadataRecord{"": {}}) == nil {
		t.Fatal("invalid persisted metadata ID was accepted")
	}
}
