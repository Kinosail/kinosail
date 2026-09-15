package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestShowPosterDoesNotShareEpisodeStillCachePath(t *testing.T) { //nolint:cyclop // Show cards must keep their poster when the first episode also has artwork.
	provider := httptest.NewServer(showArtworkProvider(nil))
	t.Cleanup(provider.Close)
	handler := showArtworkHandler(t, provider.URL)
	refresh := httptest.NewRecorder()
	handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tasks/metadata", nil))
	art := showArtwork(t, handler)
	if refresh.Code != http.StatusNoContent || art.Code != http.StatusOK || art.Body.String() != "show poster" {
		t.Fatalf("refresh = %d, art = %d %q", refresh.Code, art.Code, art.Body.String())
	}
}

func TestShowPosterDownloadFailureRemainsMissingAndRetries(t *testing.T) { //nolint:cyclop // A successful episode still cannot hide a transiently missing Show poster.
	var posterRequests atomic.Int32
	provider := httptest.NewServer(showArtworkProvider(&posterRequests))
	t.Cleanup(provider.Close)
	handler := showArtworkHandler(t, provider.URL)
	first, second := httptest.NewRecorder(), httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tasks/metadata", nil))
	handler.ServeHTTP(second, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tasks/metadata", nil))
	art := showArtwork(t, handler)
	if first.Code != http.StatusBadGateway || second.Code != http.StatusNoContent || posterRequests.Load() != 2 || art.Body.String() != "show poster" {
		t.Fatalf("first = %d %q, second = %d %q, posters = %d, art = %d %q", first.Code, first.Body.String(), second.Code, second.Body.String(), posterRequests.Load(), art.Code, art.Body.String())
	}
}

func TestShowPosterRepairDoesNotRefetchExistingEpisodes(t *testing.T) { //nolint:cyclop // Repairing one shared poster must not block on every existing episode.
	var posterAvailable atomic.Bool
	var episodeRequests atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/search/tv":
			poster := ""
			if posterAvailable.Load() {
				poster = "/show.jpg"
			}
			_, _ = fmt.Fprintf(writer, `{"results":[{"id":95396,"name":"Severance","first_air_date":"2022-02-18","poster_path":%q}]}`, poster)
		case strings.HasPrefix(request.URL.Path, "/tv/95396/season/1/episode/"):
			episodeRequests.Add(1)
			_, _ = writer.Write([]byte(`{"id":1,"name":"Episode","air_date":"2022-02-18","still_path":""}`))
		case request.URL.Path == "/show.jpg":
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write([]byte("repaired poster"))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	season := filepath.Join(mediaDir, "Severance (2022)", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	for episode := 1; episode <= 40; episode++ {
		name := fmt.Sprintf("Severance - S01E%02d.mkv", episode)
		if err := os.WriteFile(filepath.Join(season, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: t.TempDir(), Metadata: server.MetadataConfig{URL: provider.URL, ImageURL: provider.URL, Token: "token"}})
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tasks/metadata", nil))
	episodeRequests.Store(0)
	posterAvailable.Store(true)
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tasks/metadata", nil))
	shows := httptest.NewRecorder()
	handler.ServeHTTP(shows, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=shows", nil))
	art := showArtwork(t, handler)

	if first.Code != http.StatusNoContent || second.Code != http.StatusNoContent || episodeRequests.Load() != 1 || !strings.Contains(shows.Body.String(), `<img class="poster" src="/art/`) || art.Body.String() != "repaired poster" {
		t.Fatalf("first = %d, second = %d, episode requests = %d, shows = %q, art = %d %q", first.Code, second.Code, episodeRequests.Load(), shows.Body.String(), art.Code, art.Body.String())
	}
}

func showArtworkProvider(posterRequests *atomic.Int32) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/search/tv":
			_, _ = writer.Write([]byte(`{"results":[{"id":95396,"name":"Severance","first_air_date":"2022-02-18","poster_path":"/show.jpg"}]}`))
		case "/tv/95396/season/1/episode/1":
			_, _ = writer.Write([]byte(`{"id":1,"name":"Good News About Hell","air_date":"2022-02-18","still_path":"/episode.jpg"}`))
		case "/show.jpg":
			if posterRequests != nil && posterRequests.Add(1) == 1 {
				http.Error(writer, "temporary failure", http.StatusServiceUnavailable)
				return
			}
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write([]byte("show poster"))
		case "/episode.jpg":
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write([]byte("episode still"))
		default:
			http.NotFound(writer, request)
		}
	})
}

func showArtworkHandler(t *testing.T, providerURL string) http.Handler {
	t.Helper()
	mediaDir := t.TempDir()
	season := filepath.Join(mediaDir, "Severance (2022)", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(season, "Severance - S01E01.mkv"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: t.TempDir(), Metadata: server.MetadataConfig{URL: providerURL, ImageURL: providerURL, Token: "token"}})
}

func showArtwork(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	shows := httptest.NewRecorder()
	handler.ServeHTTP(shows, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/shows", nil))
	id := regexp.MustCompile(`"artwork":"/art/([a-f0-9]+)"`).FindStringSubmatch(shows.Body.String())
	if len(id) != 2 {
		t.Fatalf("shows = %d %q", shows.Code, shows.Body.String())
	}
	art := httptest.NewRecorder()
	handler.ServeHTTP(art, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/art/"+id[1], nil))
	return art
}
