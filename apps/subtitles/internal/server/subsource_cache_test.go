package server

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubSourceReusesRecentSearchResults(t *testing.T) {
	t.Parallel()
	backend := newSubSourceTestServer(t, []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"))
	t.Cleanup(backend.Close)
	var mu sync.Mutex
	requests := map[string]int{}
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		requests[request.URL.Path]++
		mu.Unlock()
		backend.Config.Handler.ServeHTTP(writer, request)
	}))
	t.Cleanup(remote.Close)
	provider := newSubSourceProvider(SubSourceConfig{URL: remote.URL + "/api/v1", APIKey: "key", PersonalUse: true})
	now := time.Now()
	provider.now = func() time.Time { return now }
	item := library.Item{ID: "arrival", Kind: "video", Path: "/media/Arrival.2016.1080p.WEB-DL.mkv", Title: "Arrival", Year: "2016", ProviderIDs: map[string]string{"imdb": "tt2543164", "tmdb": "329865"}}
	for range 3 {
		if candidates, err := provider.search(t.Context(), item, "en"); err != nil || len(candidates) != 1 {
			t.Fatalf("search = %#v, %v", candidates, err)
		}
	}
	count := func(path string) int {
		mu.Lock()
		defer mu.Unlock()
		return requests[path]
	}
	if count("/api/v1/movies/search") != 1 || count("/api/v1/subtitles") != 1 {
		t.Fatalf("repeated searches reached SubSource: %v", requests)
	}
	now = now.Add(subSourceCacheLifetime + time.Second)
	if _, err := provider.search(t.Context(), item, "en"); err != nil {
		t.Fatal(err)
	}
	if count("/api/v1/movies/search") != 2 || count("/api/v1/subtitles") != 2 {
		t.Fatalf("expired results were reused: %v", requests)
	}
}
