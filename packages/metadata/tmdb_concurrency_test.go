package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestTMDBEnrichBoundsMovieRequestsAndPreservesOrder(t *testing.T) { //nolint:cyclop,gocognit // The barrier verifies bounded overlap and item order.
	if runtime.GOMAXPROCS(0) < 4 {
		t.Skip("three workers require at least four process CPUs")
	}
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	var active, maximum atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/search/movie" {
			current := active.Add(1)
			for previous := maximum.Load(); current > previous; previous = maximum.Load() {
				if maximum.CompareAndSwap(previous, current) {
					break
				}
			}
			defer active.Add(-1)
			entered <- struct{}{}
			select {
			case <-release:
			case <-request.Context().Done():
				return
			}
			parts := strings.Fields(request.URL.Query().Get("query"))
			index, _ := strconv.Atoi(parts[len(parts)-1])
			_ = json.NewEncoder(writer).Encode(TMDBCandidates{Results: []TMDBCandidate{{ID: 100 + index}}})
			return
		}
		id := strings.TrimPrefix(request.URL.Path, "/movie/")
		_ = json.NewEncoder(writer).Encode(map[string]any{"title": "Online " + id, "release_date": "2020-01-01", "credits": map[string]any{"cast": []any{}, "crew": []any{}}})
	}))
	defer server.Close()
	client := NewTMDB(TMDBConfig{Token: "token", URL: server.URL, ImageURL: server.URL, CacheDir: t.TempDir()})
	items := make([]library.Item, 4)
	for index := range items {
		items[index] = library.Item{ID: fmt.Sprintf("movie-%d", index), Kind: "video", Title: fmt.Sprintf("Movie %d (2020)", index)}
	}
	done := make(chan []library.Item, 1)
	go func() { done <- client.Enrich(t.Context(), items) }()
	for range 3 {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("TMDB movie requests did not overlap")
		}
	}
	close(release)
	result := <-done
	if maximum.Load() != 3 {
		t.Fatalf("maximum TMDB requests = %d", maximum.Load())
	}
	for index, item := range result {
		if item.ID != fmt.Sprintf("movie-%d", index) || item.Title != fmt.Sprintf("Online %d", 100+index) {
			t.Fatalf("enriched order = %#v", result)
		}
	}
}

func TestTMDBEnrichDuplicateIDsRetainSerialCacheBehavior(t *testing.T) {
	var searches atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/search/movie" {
			searches.Add(1)
			_, _ = writer.Write([]byte(`{"results":[{"id":42}]}`))
			return
		}
		_, _ = writer.Write([]byte(`{"title":"Online","release_date":"2020-01-01","credits":{"cast":[],"crew":[]}}`))
	}))
	defer server.Close()
	client := NewTMDB(TMDBConfig{Token: "token", URL: server.URL, ImageURL: server.URL, CacheDir: t.TempDir()})
	items := client.Enrich(t.Context(), []library.Item{{ID: "same", Kind: "video", Title: "First"}, {ID: "same", Kind: "video", Title: "Second"}})
	if searches.Load() != 1 || items[0].Title != "Online" || items[1].Title != "Online" {
		t.Fatalf("duplicate enrichment = %#v, searches = %d", items, searches.Load())
	}
}

func TestTMDBCastDownloadsAreBoundedAndOrdered(t *testing.T) { //nolint:cyclop,gocognit // The barrier verifies bounded image requests and cast order.
	if runtime.GOMAXPROCS(0) < 3 {
		t.Skip("two image workers require at least three process CPUs")
	}
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	var active, maximum, requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		current := active.Add(1)
		for previous := maximum.Load(); current > previous; previous = maximum.Load() {
			if maximum.CompareAndSwap(previous, current) {
				break
			}
		}
		defer active.Add(-1)
		entered <- struct{}{}
		select {
		case <-release:
		case <-request.Context().Done():
			return
		}
		writer.Header().Set("Content-Type", "image/jpeg")
		_, _ = writer.Write([]byte("image"))
	}))
	defer server.Close()
	client := NewTMDB(TMDBConfig{Token: "token", URL: server.URL, ImageURL: server.URL, CacheDir: t.TempDir()})
	cast := []TMDBCastMember{{Name: "", ProfilePath: "/unused.jpg"}, {Name: "One", ProfilePath: "/one.jpg"}, {Name: "Two", ProfilePath: "/two.jpg"}, {Name: "Three", ProfilePath: "/three.jpg"}, {Name: "Four", ProfilePath: "/four.jpg"}}
	directory := t.TempDir()
	done := make(chan []library.Person, 1)
	go func() { done <- client.downloadCast(t.Context(), cast, directory) }()
	for range 2 {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("cast images did not overlap")
		}
	}
	close(release)
	people := <-done
	if len(people) != 4 || maximum.Load() != 2 || requests.Load() != 4 {
		t.Fatalf("cast = %#v, maximum = %d, requests = %d", people, maximum.Load(), requests.Load())
	}
	for index, person := range people {
		if person.Name != []string{"One", "Two", "Three", "Four"}[index] || person.Image == "" {
			t.Fatalf("cast order = %#v", people)
		}
	}
}

func BenchmarkTMDBEnrichMovies(b *testing.B) { //nolint:cyclop,gocognit // The benchmark compares complete uncached movie enrichment.
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		time.Sleep(20 * time.Millisecond)
		switch request.URL.Path {
		case "/search/movie":
			_, _ = writer.Write([]byte(`{"results":[{"id":42}]}`))
		case "/movie/42":
			_, _ = writer.Write([]byte(`{"title":"Online","release_date":"2020-01-01","credits":{"cast":[],"crew":[]}}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	for _, mode := range []string{"serial", "parallel"} {
		b.Run(mode, func(b *testing.B) {
			client := NewTMDB(TMDBConfig{Token: "token", URL: server.URL, ImageURL: server.URL, CacheDir: b.TempDir()})
			iteration := 0
			for b.Loop() {
				items := make([]library.Item, 4)
				for index := range items {
					items[index] = library.Item{ID: fmt.Sprintf("movie-%d-%d", iteration, index), Kind: "video", Title: "Movie (2020)"}
				}
				iteration++
				if mode == "serial" {
					for index := range items {
						metadata, err := client.fetch(b.Context(), items[index])
						if err != nil {
							b.Fatal(err)
						}
						if err := saveJSON(client.metadataPath(items[index].ID), metadata); err != nil {
							b.Fatal(err)
						}
						applyTMDB(&items[index], metadata)
					}
				} else {
					items = client.Enrich(b.Context(), items)
				}
				for _, item := range items {
					if item.Title != "Online" {
						b.Fatalf("movie was not enriched: %#v", item)
					}
				}
			}
		})
	}
}

func BenchmarkTMDBCastImages(b *testing.B) { //nolint:gocognit // The benchmark compares all cast image requests and cache writes.
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(20 * time.Millisecond)
		writer.Header().Set("Content-Type", "image/jpeg")
		_, _ = writer.Write([]byte("image"))
	}))
	defer server.Close()
	client := NewTMDB(TMDBConfig{Token: "token", URL: server.URL, ImageURL: server.URL, CacheDir: b.TempDir()})
	cast := make([]TMDBCastMember, 8)
	for index := range cast {
		cast[index] = TMDBCastMember{Name: fmt.Sprintf("Actor %d", index), ProfilePath: fmt.Sprintf("/person-%d.jpg", index)}
	}
	for _, mode := range []string{"serial", "parallel"} {
		b.Run(mode, func(b *testing.B) {
			directory := b.TempDir()
			for b.Loop() {
				var people []library.Person
				if mode == "serial" {
					for index, actor := range cast {
						image, _ := client.download(b.Context(), actor.ProfilePath, fmt.Sprintf("%s/person-%d", directory, index))
						people = append(people, library.Person{Name: actor.Name, Image: image})
					}
				} else {
					people = client.downloadCast(b.Context(), cast, directory)
				}
				if len(people) != len(cast) || people[len(people)-1].Image == "" {
					b.Fatalf("cast images = %#v", people)
				}
			}
		})
	}
}
