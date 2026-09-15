package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	sharedmetadata "github.com/MikeO7/kinosail/packages/metadata"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestMetadataRefreshUsesBoundedConcurrency(t *testing.T) {
	var active, maximum atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/search/movie" {
			current := active.Add(1)
			for current > maximum.Load() && !maximum.CompareAndSwap(maximum.Load(), current) {
			}
			time.Sleep(25 * time.Millisecond)
			active.Add(-1)
			_, _ = writer.Write([]byte(`{"results":[{"id":1,"title":"Movie"}]}`))
			return
		}
		_, _ = writer.Write([]byte(`{"belongs_to_collection":null}`))
	}))
	t.Cleanup(provider.Close)
	items := make([]library.Item, 8)
	for position := range items {
		items[position] = library.Item{ID: strconv.Itoa(position), Kind: "video", Title: fmt.Sprintf("Movie %d", position)}
	}
	store := newMetadataStore(MetadataConfig{URL: provider.URL, Token: "test"}, t.TempDir(), t.TempDir())
	index := memoryLibraryIndex(items, true)
	if err := store.refreshMissing(context.Background(), index); err != nil {
		t.Fatal(err)
	}
	if maximum.Load() < 2 || maximum.Load() > 4 || len(store.records) != len(items) {
		t.Fatalf("maximum concurrent requests = %d, records = %d", maximum.Load(), len(store.records))
	}
}

func TestMetadataSearchCleansReleaseFilename(t *testing.T) {
	title, releaseYear := sharedmetadata.MovieIdentity(library.Item{Title: "2001 A Space Odyssey (1968) [Bluray 1080p]"})
	if title != "2001 A Space Odyssey" || releaseYear != "1968" {
		t.Fatalf("identity = %q, %q", title, releaseYear)
	}
	var query, year string
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/search/movie" {
			query, year = request.URL.Query().Get("query"), request.URL.Query().Get("primary_release_year")
			_, _ = writer.Write([]byte(`{"results":[{"id":1,"title":"1917","release_date":"2019-12-25"}]}`))
			return
		}
		_, _ = writer.Write([]byte(`{"belongs_to_collection":null}`))
	}))
	t.Cleanup(provider.Close)
	store := newMetadataStore(MetadataConfig{URL: provider.URL, Token: "test"}, t.TempDir(), t.TempDir())
	_, err := store.fetchRecord(t.Context(), library.Item{Kind: "video", Title: "1917 (2019) {imdb tt8579674} [Bluray 1080p][TrueHD Atmos 7 1][x264] FuzerHD"})
	if err != nil || query != "1917" || year != "2019" {
		t.Fatalf("query = %q, year = %q, err = %v", query, year, err)
	}
}
