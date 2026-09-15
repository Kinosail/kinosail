package servertest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ServeTVMazeIdentityFixture provides one bounded trusted-origin metadata fixture.
func ServeTVMazeIdentityFixture(writer http.ResponseWriter, request *http.Request) {
	if request.Header.Get("User-Agent") == "" {
		http.Error(writer, "missing user agent", http.StatusBadRequest)
		return
	}
	if request.URL.Path == "/lookup/shows" && request.URL.Query().Get("thetvdb") == "123" {
		http.Redirect(writer, request, "/shows/42", http.StatusMovedPermanently)
		return
	}
	if request.URL.Path == "/shows/42" {
		_, _ = writer.Write([]byte(`{"id":42,"name":"Example","premiered":"2025-01-01","summary":"<p>A useful show.</p>"}`))
		return
	}
	if request.URL.Path == "/shows/42/episodebynumber" && request.URL.Query().Get("season") == "1" && request.URL.Query().Get("number") == "2" {
		_, _ = writer.Write([]byte(`{"id":9001,"name":"The Return","season":1,"number":2,"airdate":"2026-01-02","summary":"<p>A <em>second</em> chapter.</p>"}`))
		return
	}
	http.NotFound(writer, request)
}

func (suite metadataProviderContract) OwnerCanRefreshProviderMetadataAndArtwork(t *testing.T) {
	t.Parallel()
	provider := httptest.NewServer(http.HandlerFunc(MetadataProviderFixture))
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := suite.newServer(t, mediaDir, provider.URL)
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	refresh := httptest.NewRecorder()
	handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/metadata/"+id+"/refresh", nil))
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	art := httptest.NewRecorder()
	handler.ServeHTTP(art, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/art/"+id, nil))
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))

	if refresh.Code != http.StatusSeeOther || !strings.Contains(player.Body.String(), "A linguist meets visitors.") || !strings.Contains(player.Body.String(), "2016") || art.Body.String() != "poster" || !strings.Contains(settings.Body.String(), "This product uses the TMDB API") {
		t.Fatalf("refresh = %d %q, player = %q, art = %q, settings = %q", refresh.Code, refresh.Body.String(), player.Body.String(), art.Body.String(), settings.Body.String())
	}
}

func (suite metadataProviderContract) ProviderUsesShowPosterWhenEpisodeStillIsMissing(t *testing.T) {
	t.Parallel()
	provider := httptest.NewServer(metadataShowPosterHandler())
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	season := filepath.Join(mediaDir, "Severance", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(season, "Severance.S01E01.mkv"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := suite.newServer(t, mediaDir, provider.URL)
	libraryResponse := httptest.NewRecorder()
	handler.ServeHTTP(libraryResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
	id := regexp.MustCompile(`"id":"([a-f0-9]+)"`).FindStringSubmatch(libraryResponse.Body.String())[1]
	refresh := httptest.NewRecorder()
	handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/items/"+id+"/metadata/refresh", nil))
	shows := httptest.NewRecorder()
	handler.ServeHTTP(shows, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/shows", nil))
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=shows", nil))
	art := httptest.NewRecorder()
	handler.ServeHTTP(art, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/art/"+id, nil))

	if refresh.Code != http.StatusNoContent || !strings.Contains(shows.Body.String(), `"artwork":"/art/`+id+`"`) || !strings.Contains(home.Body.String(), `src="/art/`+id+`"`) || art.Code != http.StatusOK || art.Body.String() != "show poster" {
		t.Fatalf("refresh = %d, shows = %q, home = %q, art = %d %q", refresh.Code, shows.Body.String(), home.Body.String(), art.Code, art.Body.String())
	}
}

func (suite metadataProviderContract) RefreshMissingRepairsExistingShowArtwork(t *testing.T) {
	posterAvailable := false
	provider := httptest.NewServer(metadataRepairPosterHandler(&posterAvailable))
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	season := filepath.Join(mediaDir, "Severance", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(season, "Severance.S01E01.mkv"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := suite.newServer(t, mediaDir, provider.URL)
	libraryResponse := httptest.NewRecorder()
	handler.ServeHTTP(libraryResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
	id := regexp.MustCompile(`"id":"([a-f0-9]+)"`).FindStringSubmatch(libraryResponse.Body.String())[1]
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/items/"+id+"/metadata/refresh", nil))
	posterAvailable = true
	refresh := httptest.NewRecorder()
	handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tasks/metadata", nil))
	art := httptest.NewRecorder()
	handler.ServeHTTP(art, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/art/"+id, nil))

	if refresh.Code != http.StatusNoContent || art.Code != http.StatusOK || art.Body.String() != "repaired poster" {
		t.Fatalf("refresh = %d %q, art = %d %q", refresh.Code, refresh.Body.String(), art.Code, art.Body.String())
	}
}

func (suite metadataProviderContract) ProviderUsesFilenameIMDbIDBeforeTitleSearch(t *testing.T) {
	t.Parallel()
	finds, searches := 0, 0
	provider := httptest.NewServer(suite.idFirst("token", "", false, &finds, &searches))
	t.Cleanup(provider.Close)
	for adapter, expected := range map[string]int{"/metadata/%s/refresh": http.StatusSeeOther, "/api/v1/items/%s/metadata/refresh": http.StatusNoContent} {
		t.Run(adapter, func(t *testing.T) {
			mediaDir := t.TempDir()
			name := "'Twas the Night Before Christmas (1974) {imdb tt0208654} [Bluray 1080p] [DTS 1 0][x264] SADPANDA.mkv"
			if err := os.WriteFile(filepath.Join(mediaDir, name), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			handler := suite.newServer(t, mediaDir, provider.URL)
			home := httptest.NewRecorder()
			handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
			id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
			refresh := httptest.NewRecorder()
			handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf(adapter, id), nil))
			player := httptest.NewRecorder()
			handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
			if refresh.Code != expected || !strings.Contains(player.Body.String(), "A holiday special.") {
				t.Fatalf("refresh = %d, player = %q", refresh.Code, player.Body.String())
			}
		})
	}
	if finds != 2 || searches != 0 {
		t.Fatalf("finds = %d, searches = %d", finds, searches)
	}
}

func (suite metadataProviderContract) ProviderFallsBackToCleanTitleAndYearForMalformedIMDbID(t *testing.T) {
	t.Parallel()
	finds := 0
	provider := httptest.NewServer(metadataMalformedIMDbHandler(&finds))
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Primer (2004) {imdb tt123} [1080p].mkv"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := suite.newServer(t, mediaDir, provider.URL)
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	refresh := httptest.NewRecorder()
	handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/metadata/"+id+"/refresh", nil))
	if refresh.Code != http.StatusSeeOther || finds != 0 {
		t.Fatalf("refresh = %d %q, find requests = %d", refresh.Code, refresh.Body.String(), finds)
	}
}

func (suite metadataProviderContract) ProviderFallsBackToCleanTitleAndYearWhenIMDbHasNoMatch(t *testing.T) {
	t.Parallel()
	finds, searches := 0, 0
	provider := httptest.NewServer(metadataMissingIMDbHandler(&finds, &searches))
	t.Cleanup(provider.Close)
	mediaDir := t.TempDir()
	filename := "'Twas the Night Before Christmas (1974) {imdb tt0208654} [1080p].mkv"
	if err := os.WriteFile(filepath.Join(mediaDir, filename), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := suite.newServer(t, mediaDir, provider.URL)
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	refresh := httptest.NewRecorder()
	handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/metadata/"+id+"/refresh", nil))
	if refresh.Code != http.StatusSeeOther || finds != 1 || searches != 1 {
		t.Fatalf("refresh = %d %q, finds = %d, searches = %d", refresh.Code, refresh.Body.String(), finds, searches)
	}
}

func metadataShowPosterHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/severance.jpg" && request.Header.Get("Authorization") != "Bearer token" {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/search/tv":
			_, _ = writer.Write([]byte(`{"results":[{"id":95396,"name":"Severance","first_air_date":"2022-02-18","poster_path":"/severance.jpg"}]}`))
		case "/tv/95396/season/1/episode/1":
			_, _ = writer.Write([]byte(`{"id":1947720,"name":"Good News About Hell","air_date":"2022-02-18","still_path":""}`))
		case "/severance.jpg":
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write([]byte("show poster"))
		default:
			http.NotFound(writer, request)
		}
	})
}

func metadataRepairPosterHandler(posterAvailable *bool) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/search/tv":
			poster := ""
			if *posterAvailable {
				poster = "/severance.jpg"
			}
			_ = json.NewEncoder(writer).Encode(map[string]any{"results": []any{map[string]any{"id": 95396, "name": "Severance", "poster_path": poster}}})
		case "/tv/95396/season/1/episode/1":
			_, _ = writer.Write([]byte(`{"id":1947720,"name":"Good News About Hell","still_path":""}`))
		case "/severance.jpg":
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write([]byte("repaired poster"))
		default:
			http.NotFound(writer, request)
		}
	})
}

func metadataMalformedIMDbHandler(finds *int) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer token" {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/search/movie":
			if request.URL.Query().Get("query") != "Primer" || request.URL.Query().Get("primary_release_year") != "2004" {
				http.Error(writer, "unclean identity", http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"results":[{"id":14337,"title":"Primer","release_date":"2004-10-08","overview":"Two engineers discover time travel."}]}`))
		case "/movie/14337":
			_, _ = writer.Write([]byte(`{"belongs_to_collection":null}`))
		default:
			if strings.HasPrefix(request.URL.Path, "/find/") {
				(*finds)++
			}
			http.NotFound(writer, request)
		}
	})
}

func metadataMissingIMDbHandler(finds, searches *int) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer token" {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/find/tt0208654":
			(*finds)++
			_, _ = writer.Write([]byte(`{"movie_results":[]}`))
		case "/search/movie":
			(*searches)++
			if request.URL.Query().Get("query") != "'Twas the Night Before Christmas" || request.URL.Query().Get("primary_release_year") != "1974" {
				http.Error(writer, "unclean identity", http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"results":[{"id":133,"title":"'Twas the Night Before Christmas","release_date":"1974-12-08","overview":"A holiday special."}]}`))
		case "/movie/133":
			_, _ = writer.Write([]byte(`{"belongs_to_collection":null}`))
		default:
			http.NotFound(writer, request)
		}
	})
}
