package server

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/database"
	"github.com/MikeO7/kinosail/packages/library"
)

func benchmarkMetadataRecord(index int) metadataRecord {
	cast := make([]library.Person, 4)
	for position := range cast {
		cast[position] = library.Person{Name: "Actor " + strconv.Itoa(position), Role: "Role " + strconv.Itoa(position)}
	}
	return metadataRecord{Title: "Episode " + strconv.Itoa(index), Year: "2024", Plot: strings.Repeat("A long plot sentence. ", 20), Genres: "Drama", ShowTitle: "Show", ShowPlot: strings.Repeat("Show plot. ", 20), Cast: cast, ProviderIDs: map[string]string{"tmdb": strconv.Itoa(index + 1)}}
}

func BenchmarkMetadataEdit10K(b *testing.B) {
	directory := b.TempDir()
	stateDB, err := database.Open(directory, true)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = stateDB.Close() })
	store := newMetadataStore(MetadataConfig{}, directory, b.TempDir(), stateDB)
	records := make(map[string]metadataRecord, 10_000)
	for index := range 10_000 {
		records["item"+strconv.Itoa(index)] = benchmarkMetadataRecord(index)
	}
	if err := store.setAll(records); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	round := 0
	for b.Loop() {
		round++
		record := benchmarkMetadataRecord(round)
		if err := store.set("item1", record); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMetadataApplyArtwork10K(b *testing.B) {
	cache := b.TempDir()
	if err := os.MkdirAll(filepath.Join(cache, "metadata"), 0o700); err != nil {
		b.Fatal(err)
	}
	artwork := make([]string, 1_000)
	for position := range artwork {
		artwork[position] = filepath.Join(cache, "metadata", "art"+strconv.Itoa(position)+".jpg")
		if err := os.WriteFile(artwork[position], []byte("art"), 0o600); err != nil {
			b.Fatal(err)
		}
	}
	store := newMetadataStore(MetadataConfig{}, "", cache)
	items := make([]library.Item, 10_000)
	for index := range items {
		id := "item" + strconv.Itoa(index)
		record := benchmarkMetadataRecord(index)
		record.Artwork, record.Backdrop, record.ShowArtwork, record.ShowBackdrop = artwork[index%1_000], artwork[(index+1)%1_000], artwork[(index+2)%1_000], artwork[(index+3)%1_000]
		for position := range record.Cast {
			record.Cast[position].Image = artwork[(index+4+position)%1_000]
		}
		store.records[id] = record
		items[index] = library.Item{ID: id, Kind: "video", Title: id}
	}
	b.ReportAllocs()
	for b.Loop() {
		applied := store.apply(append([]library.Item(nil), items...))
		if applied[9_999].Artwork == "" {
			b.Fatal("artwork was not applied")
		}
	}
}
