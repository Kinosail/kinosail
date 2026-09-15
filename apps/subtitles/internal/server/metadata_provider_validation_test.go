package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestProviderRejectsInvalidIMDbMatchesWithoutSideEffects(t *testing.T) { //nolint:cyclop,gocognit // One table proves every untrusted provider-response boundary without persistence.
	t.Parallel()
	for name, response := range map[string]string{
		"missing":      `{}`,
		"malformed":    `{`,
		"trailing":     `{"movie_results":[{"id":1,"title":"Wrong","poster_path":"/wrong.jpg"}]}{}`,
		"out-of-range": `{"movie_results":[{"id":0,"title":"Wrong"}]}`,
		"conflicting":  `{"movie_results":[{"id":1,"title":"Wrong one"},{"id":2,"title":"Wrong two"}]}`,
		"oversized":    `{"movie_results":[{"id":1,"title":"` + strings.Repeat("x", 201) + `","poster_path":"/wrong.jpg"}]}`,
		"invalid-path": `{"movie_results":[{"id":1,"title":"Wrong","poster_path":"https://images.example/wrong.jpg"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			imageRequests := 0
			provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Header.Get("Authorization") != "Bearer token" {
					http.Error(writer, "unauthorized", http.StatusUnauthorized)
					return
				}
				switch request.URL.Path {
				case "/find/tt0208654":
					_, _ = writer.Write([]byte(response))
				case "/movie/1":
					_, _ = writer.Write([]byte(`{"belongs_to_collection":null}`))
				case "/wrong.jpg":
					imageRequests++
					writer.Header().Set("Content-Type", "image/jpeg")
					_, _ = writer.Write([]byte("wrong"))
				default:
					http.NotFound(writer, request)
				}
			}))
			t.Cleanup(provider.Close)
			mediaDir, dataDir := t.TempDir(), t.TempDir()
			filename := "'Twas the Night Before Christmas (1974) {imdb tt0208654} [1080p].mkv"
			if err := os.WriteFile(filepath.Join(mediaDir, filename), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, CacheDir: t.TempDir(), Metadata: server.MetadataConfig{URL: provider.URL, ImageURL: provider.URL, Token: "token"}})
			home := httptest.NewRecorder()
			handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
			id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
			refresh := httptest.NewRecorder()
			handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/metadata/"+id+"/refresh", nil))
			player := httptest.NewRecorder()
			handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
			metadata, _ := os.ReadFile(filepath.Join(dataDir, "metadata.json"))
			if refresh.Code != http.StatusBadGateway || imageRequests != 0 || len(metadata) != 0 || strings.Contains(player.Body.String(), "Wrong") || strings.Contains(player.Body.String(), strings.Repeat("x", 201)) {
				t.Fatalf("refresh = %d, images = %d, metadata = %q, player = %q", refresh.Code, imageRequests, metadata, player.Body.String())
			}
		})
	}
}

func TestProviderRejectsInvalidEpisodeArtworkWithoutSideEffects(t *testing.T) {
	t.Parallel()
	imageRequests := 0
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/search/tv":
			_, _ = writer.Write([]byte(`{"results":[{"id":95396,"name":"Severance","poster_path":"/poster.jpg"}]}`))
		case "/tv/95396/season/1/episode/1":
			_, _ = writer.Write([]byte(`{"id":1947720,"name":"Good News About Hell","still_path":"https://images.example/wrong.jpg"}`))
		default:
			imageRequests++
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write([]byte("wrong"))
		}
	}))
	t.Cleanup(provider.Close)
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	season := filepath.Join(mediaDir, "Severance", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(season, "Severance.S01E01.mkv"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, CacheDir: t.TempDir(), Metadata: server.MetadataConfig{URL: provider.URL, ImageURL: provider.URL, Token: "token"}})
	libraryResponse := httptest.NewRecorder()
	handler.ServeHTTP(libraryResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
	id := regexp.MustCompile(`"id":"([a-f0-9]+)"`).FindStringSubmatch(libraryResponse.Body.String())[1]
	refresh := httptest.NewRecorder()
	handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/items/"+id+"/metadata/refresh", nil))
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	metadata, _ := os.ReadFile(filepath.Join(dataDir, "metadata.json"))

	if refresh.Code != http.StatusBadGateway || imageRequests != 0 || len(metadata) != 0 || strings.Contains(player.Body.String(), "Good News About Hell") {
		t.Fatalf("refresh = %d, images = %d, metadata = %q, player = %q", refresh.Code, imageRequests, metadata, player.Body.String())
	}
}

func TestProviderRejectsInvalidShowGroupBeforeArtworkSideEffects(t *testing.T) { //nolint:cyclop // A complete two-episode group proves validation precedes every image side effect.
	t.Parallel()
	imageRequests := 0
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/search/tv":
			_, _ = writer.Write([]byte(`{"results":[{"id":95396,"name":"Severance","poster_path":"/poster.jpg"}]}`))
		case "/tv/95396/season/1/episode/1":
			_, _ = writer.Write([]byte(`{"id":1,"name":"Good News About Hell"}`))
		case "/tv/95396/season/1/episode/2":
			_, _ = writer.Write([]byte(`{"id":2,"name":"Half Loop","still_path":"https://images.example/wrong.jpg"}`))
		default:
			imageRequests++
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write([]byte("wrong"))
		}
	}))
	t.Cleanup(provider.Close)
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	season := filepath.Join(mediaDir, "Severance", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Severance.S01E01.mkv", "Severance.S01E02.mkv"} {
		if err := os.WriteFile(filepath.Join(season, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, CacheDir: t.TempDir(), Metadata: server.MetadataConfig{URL: provider.URL, ImageURL: provider.URL, Token: "token"}})
	refresh := httptest.NewRecorder()
	handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tasks/metadata", nil))
	metadata, _ := os.ReadFile(filepath.Join(dataDir, "metadata.json"))
	if refresh.Code != http.StatusBadGateway || imageRequests != 0 || len(metadata) != 0 {
		t.Fatalf("refresh = %d, images = %d, metadata = %q", refresh.Code, imageRequests, metadata)
	}
}

func TestProviderRejectsOversizedShowIdentityWithoutSideEffects(t *testing.T) {
	t.Parallel()
	providerRequests := 0
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		providerRequests++
		http.Error(writer, "unexpected provider request", http.StatusInternalServerError)
	}))
	t.Cleanup(provider.Close)
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	season := filepath.Join(mediaDir, strings.Repeat("x", 201)+" (2025)", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(season, "Oversized - S01E01.mkv"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir, CacheDir: t.TempDir(), Metadata: server.MetadataConfig{URL: provider.URL, ImageURL: provider.URL, Token: "token"}})
	refresh := httptest.NewRecorder()
	handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tasks/metadata", nil))
	metadata, _ := os.ReadFile(filepath.Join(dataDir, "metadata.json"))
	if refresh.Code != http.StatusBadGateway || providerRequests != 0 || len(metadata) != 0 {
		t.Fatalf("refresh = %d, provider requests = %d, metadata = %q", refresh.Code, providerRequests, metadata)
	}
}

func idFirstTMDB(token, prefix string, legacy bool, finds, searches *int) http.HandlerFunc { //nolint:cyclop // One fake serves both supported TMDB configuration adapters.
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+token {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case prefix + "/find/tt0208654":
			(*finds)++
			if request.URL.Query().Get("external_source") != "imdb_id" {
				http.Error(writer, "missing external source", http.StatusBadRequest)
				return
			}
			if legacy {
				_, _ = writer.Write([]byte(`{"movie_results":[{"id":133}]}`))
			} else {
				_, _ = writer.Write([]byte(`{"movie_results":[{"id":133,"title":"'Twas the Night Before Christmas","release_date":"1974-12-08","overview":"A holiday special."}]}`))
			}
		case prefix + "/search/movie":
			(*searches)++
			http.Error(writer, "title search must not run", http.StatusBadRequest)
		case prefix + "/movie/133":
			if legacy {
				_, _ = writer.Write([]byte(`{"title":"'Twas the Night Before Christmas","release_date":"1974-12-08","overview":"A holiday special."}`))
			} else {
				_, _ = writer.Write([]byte(`{"belongs_to_collection":null}`))
			}
		default:
			http.NotFound(writer, request)
		}
	}
}
