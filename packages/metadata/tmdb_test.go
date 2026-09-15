package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestTMDBEnrichesMoviesAndUsesFreshCache(t *testing.T) { //nolint:cyclop,gocognit // The assertions cover one provider workflow.
	t.Parallel()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.Header.Get("Authorization") != "Bearer token" && !strings.HasPrefix(request.URL.Path, "/image/") {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		switch request.URL.Path {
		case "/search/movie":
			if request.URL.Query().Get("query") != "Movie" || request.URL.Query().Get("primary_release_year") != "2020" || request.URL.Query().Get("include_adult") != "false" {
				t.Errorf("search query = %q", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"results":[{"id":42}]}`))
		case "/movie/42":
			_, _ = writer.Write([]byte(`{"title":"Movie Online","release_date":"2020-02-03","overview":"Plot","poster_path":"/poster.jpg","genres":[{"name":"Drama"},{"name":"Thriller"}],"credits":{"cast":[{"name":"Actor","character":"Hero","profile_path":"/actor.png"},{"name":"","character":"Ignored"}],"crew":[{"name":"Director","job":"Director"}]}}`))
		case "/image/poster.jpg", "/image/actor.png":
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write([]byte("image"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	cacheDir := t.TempDir()
	client := NewTMDB(TMDBConfig{Token: "token", URL: server.URL, ImageURL: server.URL + "/image", CacheDir: cacheDir})
	items := []library.Item{
		{ID: "movie", Kind: "video", Title: "Movie (2020)"},
		{ID: "episode", Kind: "video", Show: "Show", Title: "Episode"},
		{ID: "song", Kind: "audio", Title: "Song"},
	}
	got := client.Enrich(t.Context(), items)
	assertEnrichedTMDBMovie(t, got[0])
	if got[0].Artwork == "" || got[0].Cast[0].Image == "" {
		t.Fatalf("downloaded artwork missing: %#v", got[0])
	}
	if !client.Active() || !HasTMDBCache(filepath.Join(cacheDir, "tmdb")) {
		t.Fatal("populated TMDB cache was not active")
	}
	before := requests
	cached := client.Enrich(t.Context(), []library.Item{{ID: "movie", Kind: "video", Title: "Local", LocalTitle: true}})
	if requests != before || cached[0].Title != "Local" || cached[0].Plot != "Plot" {
		t.Fatalf("fresh cache was not used: requests=%d item=%#v", requests, cached[0])
	}
}

func assertEnrichedTMDBMovie(t *testing.T, item library.Item) {
	t.Helper()
	if item.Title != "Movie Online" || item.Year != "2020" || item.Plot != "Plot" || item.Genres != "Drama · Thriller" || item.Director != "Director" || item.ProviderIDs["tmdb"] != "42" || len(item.Cast) != 1 {
		t.Fatalf("enriched movie = %#v", item)
	}
}

func TestTMDBFetchPrefersUniqueIMDbMatch(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/find/tt1234567":
			_, _ = writer.Write([]byte(`{"movie_results":[{"id":7}]}`))
		case "/movie/7":
			_, _ = writer.Write([]byte(`{"title":"Matched","release_date":"1999-01-01","overview":"Plot","credits":{"cast":[],"crew":[]}}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client := NewTMDB(TMDBConfig{Token: "token", URL: server.URL, ImageURL: server.URL, CacheDir: t.TempDir()})
	metadata, err := client.fetch(t.Context(), library.Item{ID: "movie", Kind: "video", Title: "Wrong", Artwork: "local", ProviderIDs: map[string]string{"imdb": "tt1234567"}})
	if err != nil || metadata.Title != "Matched" || metadata.TMDBID != 7 || metadata.Poster != "" {
		t.Fatalf("metadata = %#v, %v", metadata, err)
	}
}

func TestTMDBRejectsInvalidProviderResponses(t *testing.T) { //nolint:funlen // Each case covers the same remote response boundary.
	t.Parallel()
	for name, handler := range map[string]http.HandlerFunc{
		"status":     func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusBadGateway) },
		"no results": func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte(`{"results":[]}`)) },
		"too many results": func(writer http.ResponseWriter, request *http.Request) {
			if strings.HasPrefix(request.URL.Path, "/search/") {
				results := make([]TMDBCandidate, 101)
				_ = json.NewEncoder(writer).Encode(TMDBCandidates{Results: results})
			}
		},
		"invalid movie": func(writer http.ResponseWriter, request *http.Request) {
			if strings.HasPrefix(request.URL.Path, "/search/") {
				_, _ = writer.Write([]byte(`{"results":[{"id":1}]}`))
				return
			}
			_, _ = writer.Write([]byte(`{"title":"","credits":{"cast":[]}}`))
		},
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(handler)
			defer server.Close()
			client := NewTMDB(TMDBConfig{Token: "token", URL: server.URL, ImageURL: server.URL, CacheDir: t.TempDir()})
			if _, err := client.fetch(context.Background(), library.Item{ID: "movie", Title: "Movie"}); err == nil { //nolint:usetesting // Explicit context exercises client ownership.
				t.Fatal("invalid TMDB response was accepted")
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"movie_results":[{"id":1},{"id":2}]}`))
	}))
	defer server.Close()
	client := NewTMDB(TMDBConfig{Token: "token", URL: server.URL, ImageURL: server.URL})
	if _, err := client.fetch(t.Context(), library.Item{Title: "Movie", ProviderIDs: map[string]string{"imdb": "tt1234567"}}); err == nil {
		t.Fatal("ambiguous IMDb response was accepted")
	}
}

func TestTMDBCacheValidationAndDownloadBoundaries(t *testing.T) { //nolint:cyclop,funlen,gocognit // Cache shape, paths, size, and content type are one trust boundary.
	t.Parallel()
	if NewTMDB(TMDBConfig{}) != nil {
		t.Fatal("empty TMDB configuration created a client")
	}
	for _, config := range []TMDBConfig{
		{Token: "token", URL: "http://example.com"},
		{Token: "token", URL: "https://user@example.com"},
		{Token: "token", URL: "https://example.com/a/../b"},
		{Token: "token", ImageURL: "https://example.com?x=1"},
	} {
		if NewTMDB(config) != nil {
			t.Fatalf("invalid TMDB endpoint accepted: %#v", config)
		}
	}
	cacheRoot := t.TempDir()
	client := NewTMDB(TMDBConfig{CacheDir: cacheRoot})
	if client.Active() || HasTMDBCache(filepath.Join(cacheRoot, "tmdb")) {
		t.Fatal("empty cache activity was misreported")
	}
	directory := filepath.Join(cacheRoot, "tmdb", "movie")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	poster := filepath.Join(directory, "poster.jpg")
	if err := os.WriteFile(poster, []byte("poster"), 0o600); err != nil {
		t.Fatal(err)
	}
	valid := TMDBMetadata{Title: "Movie", Year: "2020", Plot: "Plot\nSecond paragraph", Poster: poster, TMDBID: 1, Cast: []library.Person{{Name: "Actor", Role: "Role"}}}
	if err := saveJSON(filepath.Join(directory, "metadata.json"), valid); err != nil {
		t.Fatal(err)
	}
	loaded, fresh := client.Load("movie")
	if !fresh || !reflect.DeepEqual(loaded, valid) || !client.Active() {
		t.Fatalf("cache = %#v, %v", loaded, fresh)
	}
	if escaped, fresh := client.Load("../movie"); fresh || escaped.Title != "" {
		t.Fatalf("unsafe cache identity loaded: %#v, %v", escaped, fresh)
	}
	for name, metadata := range map[string]TMDBMetadata{
		"missing title": {TMDBID: 1},
		"bad year":      {Title: "Movie", Year: "20xx", TMDBID: 1},
		"bad id":        {Title: "Movie"},
		"unsafe plot":   {Title: "Movie", Plot: "hidden\x00text", TMDBID: 1},
		"outside image": {Title: "Movie", TMDBID: 1, Poster: filepath.Join(cacheRoot, "outside.jpg")},
	} {
		t.Run(name, func(t *testing.T) {
			if validCachedTMDB(metadata, directory) {
				t.Fatalf("invalid cache accepted: %#v", metadata)
			}
		})
	}
	if destination, err := cacheImage(bytes.NewReader([]byte("image")), filepath.Join(t.TempDir(), "poster.jpg")); err != nil || destination == "" {
		t.Fatalf("cache image = %q, %v", destination, err)
	}
	if _, err := cacheImage(io.LimitReader(zeroReader{}, 8<<20+1), filepath.Join(t.TempDir(), "large.jpg")); err == nil {
		t.Fatal("oversized image was accepted")
	}
}

func TestTMDBDownloadRejectsStatusAndContentType(t *testing.T) {
	t.Parallel()
	for name, handler := range map[string]http.HandlerFunc{
		"status": func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNotFound) },
		"content": func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "text/plain")
			_, _ = writer.Write([]byte("not image"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(handler)
			defer server.Close()
			client := NewTMDB(TMDBConfig{Token: "token", URL: server.URL, ImageURL: server.URL})
			if _, err := client.download(t.Context(), "/poster", filepath.Join(t.TempDir(), "poster")); err == nil {
				t.Fatal("invalid image response was accepted")
			}
		})
	}
	client := NewTMDB(TMDBConfig{Token: "token"})
	if got, err := client.download(t.Context(), "", "unused"); err != nil || got != "" {
		t.Fatalf("empty image path = %q, %v", got, err)
	}
}

func TestTMDBIdentityAndValidation(t *testing.T) { //nolint:cyclop,gocognit // The assertions cover one provider boundary.
	t.Parallel()
	if title, year := MovieIdentity(library.Item{Title: " Movie [2020] ", Year: ""}); title != "Movie" || year != "2020" {
		t.Fatalf("identity = %q, %q", title, year)
	}
	if title, year := MovieIdentity(library.Item{Title: "Movie (2020)", Year: "1999"}); title != "Movie" || year != "1999" {
		t.Fatalf("identity with local year = %q, %q", title, year)
	}
	if title, year := MovieIdentity(library.Item{Title: "1917"}); title != "1917" || year != "" {
		t.Fatalf("numeric title identity = %q, %q", title, year)
	}
	for _, id := range []string{"tt1234567", "tt123456789"} {
		if !ValidIMDbID(id) {
			t.Fatalf("valid IMDb ID %q rejected", id)
		}
	}
	for _, id := range []string{"", "tt123", "nm1234567", "tt1234567890"} {
		if ValidIMDbID(id) {
			t.Fatalf("invalid IMDb ID %q accepted", id)
		}
	}
	if !ValidTMDBPath("") || !ValidTMDBPath("/poster.jpg") || ValidTMDBPath("poster.jpg") || ValidTMDBPath("//host/path") || ValidTMDBPath("/../poster.jpg") || ValidTMDBPath("/%2e%2e/poster.jpg") || ValidTMDBPath("/"+strings.Repeat("a", 2048)) {
		t.Fatal("TMDB path validation mismatch")
	}
	if !ValidTMDBCast([]TMDBCastMember{{Name: "Actor", ProfilePath: "/actor.jpg"}}) || ValidTMDBCast([]TMDBCastMember{{Name: strings.Repeat("x", 201)}}) {
		t.Fatal("TMDB cast validation mismatch")
	}
	if got := metadataFor(tmdbMovie{Title: " Movie ", ReleaseDate: "2020-01-01", Overview: " Plot "}); got.Title != "Movie" || got.Year != "2020" || got.Plot != "Plot" {
		t.Fatalf("normalized metadata = %#v", got)
	}
}

type zeroReader struct{}

func (zeroReader) Read(buffer []byte) (int, error) {
	for index := range buffer {
		buffer[index] = 0
	}
	return len(buffer), nil
}

func TestApplyTMDBPreservesLocalFields(t *testing.T) {
	t.Parallel()
	item := library.Item{Title: "Local", LocalTitle: true, Year: "1999", Plot: "Local", Genres: "Local", Director: "Local", Artwork: "local", Cast: []library.Person{{Name: "Local"}}}
	metadata := TMDBMetadata{Title: "Remote", Year: "2020", Plot: "Remote", Genres: "Remote", Director: "Remote", Poster: "remote", Cast: []library.Person{{Name: "Remote"}}, TMDBID: 7}
	applyTMDB(&item, metadata)
	if item.Title != "Local" || item.Year != "1999" || item.Plot != "Local" || item.Artwork != "local" || item.Cast[0].Name != "Local" || item.ProviderIDs["tmdb"] != "7" {
		t.Fatalf("local fields overwritten: %#v", item)
	}
}
