package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestShowMetadataUsesFilenameIMDbIDAndDownloadsPosterOnce(t *testing.T) { //nolint:cyclop // One public task proves Show identity, provider lookup, caching, API, web, and artwork delivery.
	finds, posters := 0, 0
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/tumble.jpg" && request.Header.Get("Authorization") != "Bearer token" {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/find/tt2948562":
			finds++
			_, _ = writer.Write([]byte(`{"tv_results":[{"id":65047,"name":"Tumble Leaf","first_air_date":"2013-04-19","poster_path":"/tumble.jpg"}]}`))
		case "/tv/65047/season/1/episode/1":
			_, _ = writer.Write([]byte(`{"id":1,"name":"Shiny Coin","air_date":"2013-04-19","still_path":""}`))
		case "/tv/65047/season/1/episode/2":
			_, _ = writer.Write([]byte(`{"id":2,"name":"Fig Finds a Key","air_date":"2013-04-19","still_path":""}`))
		case "/tumble.jpg":
			posters++
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write([]byte("show poster"))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	season := filepath.Join(mediaDir, "Tumble Leaf (2014) {imdb-tt2948562}", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Tumble Leaf - S01E01.mkv", "Tumble Leaf - S01E02.mkv"} {
		if err := os.WriteFile(filepath.Join(season, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(season, "Tumble Leaf - S01E01.nfo"), []byte(`<episodedetails><title>Curated Coin</title><plot>Keep this local plot.</plot></episodedetails>`), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: t.TempDir(), Metadata: server.MetadataConfig{URL: provider.URL, ImageURL: provider.URL, Token: "token"}})
	refresh := httptest.NewRecorder()
	handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tasks/metadata", nil))
	shows := httptest.NewRecorder()
	handler.ServeHTTP(shows, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/shows", nil))
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=shows", nil))
	recent := httptest.NewRecorder()
	handler.ServeHTTP(recent, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`"artwork":"/art/([a-f0-9]+)"`).FindStringSubmatch(shows.Body.String())
	if len(id) != 2 {
		t.Fatalf("shows = %d %q", shows.Code, shows.Body.String())
	}
	art := httptest.NewRecorder()
	handler.ServeHTTP(art, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/art/"+id[1], nil))

	if refresh.Code != http.StatusNoContent || finds != 1 || posters != 1 || !strings.Contains(shows.Body.String(), `"title":"Tumble Leaf"`) || !strings.Contains(home.Body.String(), `>Tumble Leaf</h2>`) || !strings.Contains(recent.Body.String(), `class="card recent-card stacked" href="/show/`) || !strings.Contains(recent.Body.String(), `<img src="/art/`) || !strings.Contains(recent.Body.String(), `2 episodes stacked`) || art.Code != http.StatusOK || art.Body.String() != "show poster" {
		t.Fatalf("refresh = %d, finds = %d, posters = %d, shows = %q, home = %q, recent = %q, art = %d %q", refresh.Code, finds, posters, shows.Body.String(), home.Body.String(), recent.Body.String(), art.Code, art.Body.String())
	}
}

func TestShowMetadataFallsBackFromRegionSuffixToUniqueYearMatch(t *testing.T) { //nolint:cyclop,gocognit // The live malformed-ID pattern remains strict while recovering a unique provider match.
	searches, posters := 0, 0
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/golden.jpg" && request.Header.Get("Authorization") != "Bearer token" {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/search/tv":
			searches++
			if request.URL.Query().Get("query") == "The Golden Bachelor (AU)" && request.URL.Query().Get("first_air_date_year") == "2025" {
				_, _ = writer.Write([]byte(`{"results":[]}`))
				return
			}
			if request.URL.Query().Get("query") == "The Golden Bachelor" && request.URL.Query().Get("first_air_date_year") == "2025" {
				_, _ = writer.Write([]byte(`{"results":[{"id":303516,"name":"The Golden Bachelor Australia","first_air_date":"2025-10-20","poster_path":"/golden.jpg"}]}`))
				return
			}
			http.Error(writer, "unexpected search", http.StatusBadRequest)
		case "/tv/303516/season/1/episode/1":
			_, _ = writer.Write([]byte(`{"id":1,"name":"Episode 1","air_date":"2025-10-20","still_path":""}`))
		case "/golden.jpg":
			posters++
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write([]byte("golden poster"))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	season := filepath.Join(mediaDir, "The Golden Bachelor (AU) (2025) {imdb-}", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(season, "The Golden Bachelor - S01E01.mkv"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: t.TempDir(), Metadata: server.MetadataConfig{URL: provider.URL, ImageURL: provider.URL, Token: "token"}})
	refresh := httptest.NewRecorder()
	handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tasks/metadata", nil))
	shows := httptest.NewRecorder()
	handler.ServeHTTP(shows, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/shows", nil))
	if refresh.Code != http.StatusNoContent || searches != 2 || posters != 1 || !strings.Contains(shows.Body.String(), `"title":"The Golden Bachelor Australia"`) || !strings.Contains(shows.Body.String(), `"artwork":"/art/`) {
		t.Fatalf("refresh = %d, searches = %d, posters = %d, shows = %q", refresh.Code, searches, posters, shows.Body.String())
	}
}

func TestShowMetadataFallsBackFromWrongYearToUniqueTitle(t *testing.T) { //nolint:cyclop,gocognit // A uniquely named Show can recover from a stale folder year without weakening ambiguous matching.
	searches := 0
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/tumble.jpg" && request.Header.Get("Authorization") != "Bearer token" {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/search/tv":
			searches++
			if request.URL.Query().Get("query") != "Tumble Leaf" {
				http.Error(writer, "unexpected query", http.StatusBadRequest)
				return
			}
			if request.URL.Query().Get("first_air_date_year") == "2014" {
				_, _ = writer.Write([]byte(`{"results":[]}`))
				return
			}
			if request.URL.Query().Get("first_air_date_year") == "" {
				_, _ = writer.Write([]byte(`{"results":[{"id":65047,"name":"Tumble Leaf","first_air_date":"2013-04-19","poster_path":"/tumble.jpg"}]}`))
				return
			}
			http.Error(writer, "unexpected year", http.StatusBadRequest)
		case "/tv/65047/season/1/episode/1":
			_, _ = writer.Write([]byte(`{"id":1,"name":"Shiny Coin","air_date":"2013-04-19","still_path":""}`))
		case "/tumble.jpg":
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write([]byte("show poster"))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	season := filepath.Join(mediaDir, "Tumble Leaf (2014)", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(season, "Tumble Leaf - S01E01.mkv"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: t.TempDir(), Metadata: server.MetadataConfig{URL: provider.URL, ImageURL: provider.URL, Token: "token"}})
	refresh := httptest.NewRecorder()
	handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tasks/metadata", nil))
	shows := httptest.NewRecorder()
	handler.ServeHTTP(shows, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/shows", nil))
	if refresh.Code != http.StatusNoContent || searches != 2 || !strings.Contains(shows.Body.String(), `"title":"Tumble Leaf"`) || !strings.Contains(shows.Body.String(), `"artwork":"/art/`) {
		t.Fatalf("refresh = %d, searches = %d, shows = %q", refresh.Code, searches, shows.Body.String())
	}
}

func TestShowMetadataRejectsAmbiguousFallbackWithoutSideEffects(t *testing.T) { //nolint:cyclop // Ambiguous provider data cannot write metadata or artwork.
	images := 0
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/search/tv":
			if request.URL.Query().Get("query") == "The Golden Bachelor (AU)" {
				_, _ = writer.Write([]byte(`{"results":[]}`))
				return
			}
			_, _ = writer.Write([]byte(`{"results":[{"id":1,"name":"Candidate One"},{"id":2,"name":"Candidate Two"}]}`))
		case "/one.jpg", "/two.jpg":
			images++
			writer.Header().Set("Content-Type", "image/jpeg")
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	season := filepath.Join(mediaDir, "The Golden Bachelor (AU) (2025) {imdb-}", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(season, "The Golden Bachelor - S01E01.mkv"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: t.TempDir(), Metadata: server.MetadataConfig{URL: provider.URL, ImageURL: provider.URL, Token: "token"}})
	refresh := httptest.NewRecorder()
	handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tasks/metadata", nil))
	shows := httptest.NewRecorder()
	handler.ServeHTTP(shows, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/shows", nil))
	if refresh.Code != http.StatusBadGateway || images != 0 || strings.Contains(shows.Body.String(), `"artwork":`) || !strings.Contains(shows.Body.String(), `"title":"The Golden Bachelor (AU)"`) {
		t.Fatalf("refresh = %d %q, images = %d, shows = %q", refresh.Code, refresh.Body.String(), images, shows.Body.String())
	}
}

func TestShowMetadataPersistsSuccessfulGroupsWhenAnotherGroupFails(t *testing.T) { //nolint:cyclop // A partial provider failure remains visible without discarding valid independent work.
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/search/tv":
			switch request.URL.Query().Get("query") {
			case "Bluey":
				_, _ = writer.Write([]byte(`{"results":[{"id":82728,"name":"Bluey","first_air_date":"2018-10-01","poster_path":"/bluey.jpg"}]}`))
			case "Mystery (AU)":
				_, _ = writer.Write([]byte(`{"results":[]}`))
			case "Mystery":
				_, _ = writer.Write([]byte(`{"results":[{"id":1,"name":"Mystery One"},{"id":2,"name":"Mystery Two"}]}`))
			default:
				http.Error(writer, "unexpected search", http.StatusBadRequest)
			}
		case "/tv/82728/season/1/episode/1":
			_, _ = writer.Write([]byte(`{"id":1,"name":"Magic Xylophone","air_date":"2018-10-01","still_path":""}`))
		case "/bluey.jpg":
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write([]byte("bluey poster"))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	for _, path := range []string{
		"Bluey (2018)/Season 01/Bluey - S01E01.mkv",
		"Mystery (AU) (2025)/Season 01/Mystery - S01E01.mkv",
	} {
		path = filepath.Join(mediaDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: t.TempDir(), Metadata: server.MetadataConfig{URL: provider.URL, ImageURL: provider.URL, Token: "token"}})
	refresh := httptest.NewRecorder()
	handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tasks/metadata", nil))
	shows := httptest.NewRecorder()
	handler.ServeHTTP(shows, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/shows", nil))
	if refresh.Code != http.StatusBadGateway || !strings.Contains(refresh.Body.String(), "failed for 1 of 2 items") || !strings.Contains(shows.Body.String(), `"title":"Bluey","artwork":"/art/`) || !strings.Contains(shows.Body.String(), `"title":"Mystery (AU)"`) {
		t.Fatalf("refresh = %d %q, shows = %q", refresh.Code, refresh.Body.String(), shows.Body.String())
	}
}
