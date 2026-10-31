package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sync"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubDLEpisodeSearchUsesTheShowYearAndIDs(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var query url.Values
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		query = request.URL.Query()
		mu.Unlock()
		_, _ = writer.Write([]byte(`{"status":true,"results":[],"subtitles":[]}`))
	}))
	t.Cleanup(remote.Close)
	item := library.Item{
		ID: "0123456789abcdef", Kind: "video", Title: "The Long Night", Year: "2019", Path: filepath.Join(t.TempDir(), "Show.S05E03.mkv"),
		ProviderIDs: map[string]string{"imdb": "tt-episode"}, Show: "The Show", ShowYear: "2014", Season: 5, Episode: 3,
		ShowProviderIDs: map[string]string{"tmdb": "1399"},
	}
	provider := newSubtitleProvider(SubtitleConfig{URL: remote.URL, APIKey: "key"}, t.TempDir(), t.TempDir(), sidecarTestIndex(item), nil, "")
	if _, err := provider.subdl.search(t.Context(), item, "en"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if query.Get("film_name") != "The Show" || query.Get("year") != "2014" || query.Get("tmdb_id") != "1399" || query.Has("imdb_id") || query.Get("type") != "tv" || query.Get("season_number") != "5" || query.Get("episode_number") != "3" {
		t.Fatalf("SubDL episode query = %v", query)
	}
}
