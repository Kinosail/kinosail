package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func BenchmarkFetchWantedStopsAtLimit(b *testing.B) {
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(writer).Encode(subDLResponse{Status: true})
	}))
	b.Cleanup(remote.Close)
	items := make([]library.Item, 10_000)
	tracks := []string{"Movie.en.srt", "Movie.es.srt", "Movie.fr.srt", "Movie.de.srt", "Movie.it.srt", "Movie.nl.srt", "Movie.pl.srt", "Movie.ru.srt", "Movie.uk.srt", "Movie.tr.srt"}
	for index := range items {
		items[index] = library.Item{ID: "item", Kind: "video", Path: "/media/Movie.mkv", Title: "Movie", Subtitles: tracks}
	}
	items[0].Subtitles = nil
	index := memoryLibraryIndex(items, true)
	settings := newSettingsStore("", "", "", nil)
	provider := newSubtitleProvider(SubtitleConfig{URL: remote.URL, APIKey: "key"}, b.TempDir(), b.TempDir(), index, settings, "")
	manager := newSubtitleManager(index, settings, provider, nil)
	request := ownerRequest("/")
	b.ReportAllocs()
	b.ReportMetric(float64(len(items)), "library-items")
	for b.Loop() {
		attempted, written, err := manager.fetchWantedLanguages(request, []string{"en"}, 1)
		if attempted != 1 || written != 0 || err == nil {
			b.Fatalf("attempted=%d written=%d error=%v", attempted, written, err)
		}
	}
}
