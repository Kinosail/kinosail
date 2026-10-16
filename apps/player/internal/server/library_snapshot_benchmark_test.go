package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

const snapshotBenchmarkCount = 20_000

func snapshotBenchmarkItems(count int) []library.Item {
	items := make([]library.Item, 0, count)
	added := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ratings := []string{"PG", "PG-13", "R"}
	for position := 0; len(items) < count; position++ {
		id := fmt.Sprintf("%016x", position)
		item := library.Item{ID: id, Library: "Library", Kind: "video", Rating: ratings[position%len(ratings)], Container: "MP4", Year: "2026", Added: added.Add(time.Duration(position) * time.Minute), Genres: []string{"Drama", "Comedy", "Action"}[position%3]}
		switch {
		case position%10 < 4:
			item.Title = fmt.Sprintf("Movie %06d", position)
		case position%10 < 9:
			show := position / 250
			item.Show, item.Season, item.Episode = fmt.Sprintf("Show %04d", show), 1+position%2, position%250
			item.Title = fmt.Sprintf("S%02dE%03d · Episode", item.Season, item.Episode)
		default:
			item.Kind, item.Container = "audio", "MP3"
			item.Album, item.Artist, item.Track = fmt.Sprintf("Album %04d", position/100), "Artist", position%100
			item.Title = fmt.Sprintf("Track %06d", position)
		}
		items = append(items, item)
	}
	return items
}

func snapshotBenchmarkRequest(ctx context.Context, target string, owner bool) *http.Request {
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	profile := viewerProfile{ID: "owner", Owner: true}
	if !owner {
		profile = viewerProfile{ID: "family", Rating: "teen", Libraries: []string{"all"}}
	}
	return request.WithContext(context.WithValue(request.Context(), viewerContextKey{}, profile))
}

func snapshotBenchmarkMux(b *testing.B, items []library.Item, media ...int) *http.ServeMux {
	directory := b.TempDir()
	artwork := filepath.Join(directory, "poster.jpg")
	if err := os.WriteFile(artwork, []byte("poster"), 0o600); err != nil {
		b.Fatal(err)
	}
	for position := range items {
		if items[position].Show != "" {
			items[position].ShowArtwork = artwork
		}
	}
	for _, position := range media {
		items[position].Path = filepath.Join(directory, items[position].ID+".media")
		if err := os.WriteFile(items[position].Path, []byte("media"), 0o600); err != nil {
			b.Fatal(err)
		}
	}
	index := memoryLibraryIndex(items, true)
	index.SetRoots([]libraryRoot{{Path: directory, Namespace: "Library"}})
	progress, lists, settings := newProgressStore(""), newListStore(""), newSettingsStore("", "", "", nil)
	probe := newMediaProbe("")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /watch/{id}", watch(index, progress, settings, lists, probe, newMetadataStore(MetadataConfig{}, "", ""), newWatchRooms(time.Hour, index, nil)))
	mux.HandleFunc("GET /show/{id}", browseShow(index, progress))
	mux.HandleFunc("GET /{$}", showHome(index, progress, lists, settings, nil, false))
	registerMediaAPI(mux, index, progress, lists, &authentication{}, nil)
	api := &jellyfinAPI{settings: settings, index: index, progress: progress, lists: lists, probe: probe}
	api.registerItems(mux)
	api.registerPlayback(mux)
	return mux
}

func BenchmarkLibrarySnapshotRequests(b *testing.B) {
	items := snapshotBenchmarkItems(snapshotBenchmarkCount)
	middle := snapshotBenchmarkCount / 2
	mux := snapshotBenchmarkMux(b, items, middle, middle+5, middle+9)
	movie, episode, track := items[middle], items[middle+5], items[middle+9]
	show := library.ShowID(episode.Show)
	for _, test := range []struct{ name, target string }{
		{"watch-episode", "/watch/" + episode.ID},
		{"watch-movie", "/watch/" + movie.ID},
		{"watch-track", "/watch/" + track.ID},
		{"show-page", "/show/" + show},
		{"api-show", "/api/v1/shows/" + show},
		{"home-landing", "/"},
		{"jellyfin-items-page", "/Items?startIndex=10000&limit=50"},
		{"jellyfin-items-search", "/Items?searchTerm=movie%20001&limit=50"},
		{"jellyfin-item", "/Items/" + movie.ID + "0000000000000000"},
		{"jellyfin-series-image", "/Items/" + show + "0000000000000000/Images/Primary"},
	} {
		for _, owner := range []bool{true, false} {
			b.Run(fmt.Sprintf("%s/owner=%t", test.name, owner), func(b *testing.B) {
				request := snapshotBenchmarkRequest(b.Context(), test.target, owner)
				b.ReportAllocs()
				for b.Loop() {
					response := httptest.NewRecorder()
					mux.ServeHTTP(response, request)
					if response.Code != http.StatusOK {
						b.Fatalf("%s = %d", test.target, response.Code)
					}
				}
			})
		}
	}
}
