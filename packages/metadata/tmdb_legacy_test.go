package metadata

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func writeLegacyTMDB(t *testing.T, cache, id string, metadata TMDBMetadata) {
	t.Helper()
	directory := filepath.Join(cache, "tmdb", id)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if metadata.Poster != "" {
		metadata.Poster = filepath.Join(directory, metadata.Poster)
		if err := os.WriteFile(metadata.Poster, []byte("poster"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "metadata.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyTMDBCacheMigratesOnceIntoMetadataRecords(t *testing.T) {
	cache := t.TempDir()
	writeLegacyTMDB(t, cache, "primer", TMDBMetadata{Title: "Primer", Year: "2004", Plot: "Engineers.", Rating: "PG-13", Genres: "Science Fiction", Poster: "poster.jpg", TMDBID: 14337, Version: 2, Cast: []library.Person{{Name: "Shane Carruth"}}})
	writeLegacyTMDB(t, cache, "known", TMDBMetadata{Title: "Known", TMDBID: 1, Version: 2})
	writeLegacyTMDB(t, cache, "invalid", TMDBMetadata{Title: "", TMDBID: 2})
	writeLegacyTMDB(t, cache, "outside", TMDBMetadata{Title: "Arrival", TMDBID: 3, Poster: "../../../outside.jpg"})
	writeLegacyTMDB(t, cache, "oversized", TMDBMetadata{Title: strings.Repeat("x", 201), TMDBID: 4})
	trailing := filepath.Join(cache, "tmdb", "trailing")
	if err := os.MkdirAll(trailing, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(trailing, "metadata.json"), []byte(`{"Title":"Arrival","TMDBID":5}{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var saved map[string]Record
	known := func(id string) bool { return id == "known" }
	if !MigrateLegacyTMDB(cache, known, func(updates map[string]Record) error { saved = updates; return errors.New("disk full") }, true) {
		t.Fatal("legacy cache was not detected")
	}
	if _, err := os.Stat(filepath.Join(cache, "tmdb")); err != nil {
		t.Fatalf("failed migration removed the legacy cache: %v", err)
	}
	if !MigrateLegacyTMDB(cache, known, func(updates map[string]Record) error { saved = updates; return nil }, true) {
		t.Fatal("legacy cache was not migrated")
	}
	record := saved["primer"]
	if len(saved) != 1 || record.Title != "Primer" || record.Year != "2004" || record.Rating != "PG-13" || record.ProviderIDs["tmdb"] != "14337" || record.Artwork != filepath.Join(cache, "metadata", "primer.jpg") || record.CastFetched {
		t.Fatalf("migrated records = %#v", saved)
	}
	if data, err := os.ReadFile(record.Artwork); err != nil || string(data) != "poster" {
		t.Fatalf("migrated poster = %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(cache, "tmdb")); !os.IsNotExist(err) {
		t.Fatalf("durable migration kept the legacy cache: %v", err)
	}
	if MigrateLegacyTMDB(cache, known, func(map[string]Record) error { t.Fatal("migrated twice"); return nil }, true) {
		t.Fatal("removed legacy cache was migrated again")
	}
}
