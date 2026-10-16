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

type snapshotFixture struct {
	mux      *http.ServeMux
	index    *libraryIndex
	progress *progressStore
}

func snapshotFixtureItems(count int) []library.Item {
	items := make([]library.Item, 0, count)
	added := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ratings := []string{"PG", "PG-13", "R"}
	for position := 0; len(items) < count; position++ {
		id := fmt.Sprintf("%016x", position)
		item := library.Item{ID: id, Library: "Library", Kind: "video", Rating: ratings[position%len(ratings)], Container: "MP4", Year: "2026", Added: added.Add(time.Duration(position) * time.Minute), Genres: []string{"Drama", "Comedy", "Action"}[position%3], Collection: []string{"", "Saga"}[position%7/6]}
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

func snapshotRequest(ctx context.Context, method, target string, profile viewerProfile) *http.Request {
	return httptest.NewRequestWithContext(context.WithValue(ctx, viewerContextKey{}, profile), method, target, nil)
}

func newSnapshotFixture(t testing.TB, items []library.Item, media ...int) snapshotFixture {
	directory := t.TempDir()
	artwork := filepath.Join(directory, "poster.jpg")
	if err := os.WriteFile(artwork, []byte("poster"), 0o600); err != nil {
		t.Fatal(err)
	}
	for position := range items {
		if items[position].Show != "" {
			items[position].ShowArtwork = artwork
		}
	}
	for _, position := range media {
		items[position].Path = filepath.Join(directory, items[position].ID+".media")
		if err := os.WriteFile(items[position].Path, []byte("media"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	index := memoryLibraryIndex(items, true)
	index.SetRoots([]libraryRoot{{Path: directory, Namespace: "Library"}})
	progress, lists, settings := newProgressStore(""), newListStore(""), newSettingsStore("", "", "", nil)
	probe := newMediaProbe("")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /watch/{id}", watch(index, progress, settings, lists, probe, newMetadataStore(MetadataConfig{}, "", ""), newWatchRooms(time.Hour, index, nil)))
	mux.HandleFunc("GET /{$}", showHome(index, progress, lists, settings, nil, false))
	mux.HandleFunc("GET /api/v1/library", func(writer http.ResponseWriter, request *http.Request) {
		projectionBenchmarkHandler(index, progress, lists)(writer, request)
	})
	registerBrowsers(mux, index, progress, lists)
	registerMediaAPI(mux, index, progress, lists, &authentication{}, nil)
	api := &jellyfinAPI{settings: settings, index: index, progress: progress, lists: lists, probe: probe}
	api.registerItems(mux)
	api.registerPlayback(mux)
	return snapshotFixture{mux: mux, index: index, progress: progress}
}
