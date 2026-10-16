package server

import (
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
