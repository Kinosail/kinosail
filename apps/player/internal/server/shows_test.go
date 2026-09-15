package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestViewerCanBrowseAShowsEpisodes(t *testing.T) {
	t.Parallel()

	servertest.ViewerCanBrowseAShowsEpisodes(t, func(t *testing.T, media string) http.Handler {
		return newJellyfinServer(t, server.Config{MediaDir: media})
	}, `class="episode-index" aria-hidden="true">01`, ">Good News</strong>", `aria-label="Play again episode 1, Good News, watched"`)
}

func TestEpisodeStillUsesItsArtworkVariantWithoutReplacingShowPoster(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	showDir := filepath.Join(mediaDir, "Example Show")
	season := filepath.Join(showDir, "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(showDir, "poster.jpg"):                          "show poster",
		filepath.Join(season, "Example Show S01E01 Pilot.mp4"):        "episode",
		filepath.Join(season, "Example Show S01E01 Pilot-thumb.webp"): "episode still",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	handler := server.New(server.Config{MediaDir: mediaDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=shows", nil))
	showID := regexp.MustCompile(`/show/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	show := httptest.NewRecorder()
	handler.ServeHTTP(show, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/"+showID, nil))
	episodeID := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(show.Body.String())[1]
	assertEpisodeArtworkVariants(t, handler, show.Body.String(), showID, episodeID)
}

func assertEpisodeArtworkVariants(t *testing.T, handler http.Handler, showMarkup, showID, episodeID string) {
	t.Helper()
	poster, still, invalid, duplicate, unknown, api := httptest.NewRecorder(), httptest.NewRecorder(), httptest.NewRecorder(), httptest.NewRecorder(), httptest.NewRecorder(), httptest.NewRecorder()
	handler.ServeHTTP(poster, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/art/"+episodeID, nil))
	handler.ServeHTTP(still, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/art/"+episodeID+"?variant=episode", nil))
	handler.ServeHTTP(invalid, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/art/"+episodeID+"?variant=unknown", nil))
	handler.ServeHTTP(duplicate, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/art/"+episodeID+"?variant=episode&variant=episode", nil))
	handler.ServeHTTP(unknown, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/art/"+episodeID+"?cache=1", nil))
	handler.ServeHTTP(api, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/shows/"+showID, nil))

	assertEpisodeArtworkBody(t, poster, "show poster")
	assertEpisodeArtworkBody(t, still, "episode still")
	for _, response := range []*httptest.ResponseRecorder{invalid, duplicate, unknown} {
		if response.Code != http.StatusNotFound {
			t.Fatalf("invalid artwork variant = %d %q", response.Code, response.Body.String())
		}
	}
	if !strings.Contains(showMarkup, `class="episode-preview-art"`) || !strings.Contains(showMarkup, `/art/`+episodeID+`?variant=episode`) || !strings.Contains(api.Body.String(), `"artwork":"/art/`+episodeID+`?variant=episode"`) {
		t.Fatalf("show = %q, api = %q", showMarkup, api.Body.String())
	}
}

func assertEpisodeArtworkBody(t *testing.T, response *httptest.ResponseRecorder, expected string) {
	t.Helper()
	if response.Code != http.StatusOK || response.Body.String() != expected {
		t.Fatalf("artwork = %d %q, want %q", response.Code, response.Body.String(), expected)
	}
}

func TestShowsBrowseCountsAndJumpsByShow(t *testing.T) { //nolint:cyclop // One browse contract covers counts, grouping, and title jumps.
	t.Parallel()

	mediaDir := t.TempDir()
	for _, name := range []string{
		"Alpha S01E01 Pilot.mkv",
		"Alpha S01E02 Finale.mkv",
		"Zulu S01E01 Arrival.mkv",
	} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	response := httptest.NewRecorder()
	server.New(server.Config{MediaDir: mediaDir}).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=shows", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "<strong>2</strong> items") || strings.Count(body, `class="card show-card"`) != 2 || !strings.Contains(body, `data-title-letter="A"`) || !strings.Contains(body, `data-title-letter="Z"`) {
		t.Fatalf("shows browse = %d %q", response.Code, body)
	}
	apiResponse := httptest.NewRecorder()
	server.New(server.Config{MediaDir: mediaDir}).ServeHTTP(apiResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library?view=shows", nil))
	var page struct {
		Items   []struct{ Title string } `json:"items"`
		Letters []struct{ Label string } `json:"letters"`
		Total   int                      `json:"total"`
	}
	if err := json.Unmarshal(apiResponse.Body.Bytes(), &page); err != nil || apiResponse.Code != http.StatusOK || page.Total != 2 || len(page.Items) != 2 || len(page.Letters) != 2 || page.Letters[0].Label != "A" || page.Letters[1].Label != "Z" {
		t.Fatalf("shows API = %d, error = %v, page = %#v, body = %q", apiResponse.Code, err, page, apiResponse.Body.String())
	}
	response = httptest.NewRecorder()
	server.New(server.Config{MediaDir: mediaDir}).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=shows&letter=Z", nil))
	body = response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "<strong>2</strong> items") || !strings.Contains(body, ">Zulu<") || strings.Contains(body, ">Alpha<") {
		t.Fatalf("shows letter browse = %d %q", response.Code, body)
	}
}

func TestShowCardsPlayNextAndResume(t *testing.T) {
	t.Parallel()
	season := filepath.Join(t.TempDir(), "Severance", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(season, "Severance.S01E01.Good.News.mkv"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: filepath.Dir(filepath.Dir(season))})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=shows", nil))
	episodeID := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	if !strings.Contains(home.Body.String(), `class="card show-card"`) || !strings.Contains(home.Body.String(), `class="show-play"`) || !strings.Contains(home.Body.String(), `>Play next</span>`) {
		t.Fatalf("home has no direct show playback: %q", home.Body.String())
	}
	progress := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/progress/"+episodeID, strings.NewReader("seconds=42"))
	progress.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), progress)
	resume, api := httptest.NewRecorder(), httptest.NewRecorder()
	handler.ServeHTTP(resume, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=shows", nil))
	handler.ServeHTTP(api, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/shows", nil))
	if !strings.Contains(resume.Body.String(), `>Resume</span>`) || !strings.Contains(api.Body.String(), `"play":{"id":"`+episodeID+`","title":"S01E01 · Good News","label":"Resume","stream":"/watch/`+episodeID+`"}`) {
		t.Fatalf("resume action missing: home=%q api=%q", resume.Body.String(), api.Body.String())
	}
}

func TestViewerCanLoadLocalShowArtworkWithoutEpisodeArtwork(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	season := filepath.Join(mediaDir, "Severance", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(mediaDir, "Severance", "poster.jpg"):      "show poster",
		filepath.Join(season, "Severance.S01E01.Good.News.mkv"): "episode",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := newJellyfinServer(t, server.Config{MediaDir: mediaDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=shows", nil))
	episodeID := regexp.MustCompile(`/art/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	showID := regexp.MustCompile(`/show/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	art := httptest.NewRecorder()
	handler.ServeHTTP(art, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/art/"+episodeID, nil))
	seriesArt := httptest.NewRecorder()
	handler.ServeHTTP(seriesArt, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/Items/"+showID+strings.Repeat("0", 16)+"/Images/Primary", nil))

	if art.Code != http.StatusOK || art.Body.String() != "show poster" || seriesArt.Code != http.StatusOK || seriesArt.Body.String() != "show poster" {
		t.Fatalf("web art = %d %q, series art = %d %q", art.Code, art.Body.String(), seriesArt.Code, seriesArt.Body.String())
	}
}
