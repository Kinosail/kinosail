package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestHomeShowsRecentlyAddedInNewestFirstOrder(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	old := filepath.Join(mediaDir, "Old.mp4")
	newer := filepath.Join(mediaDir, "New.mp4")
	for _, path := range []string{old, newer} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(old, time.Unix(1, 0), time.Unix(1, 0)); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.New(server.Config{MediaDir: mediaDir}).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	body := response.Body.String()
	start, end := strings.Index(body, "Recently added"), strings.Index(body, `<section class="destination-browser"`)
	if start < 0 || end < 0 {
		t.Fatalf("home has no recent shelf: %q", body)
	}
	recent := body[start:end]

	if strings.Index(recent, ">New<") > strings.Index(recent, ">Old<") {
		t.Fatalf("recent = %q", recent)
	}
}

func TestHomePrioritizesVisibleRecentArtwork(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	for position := range 3 {
		for _, extension := range []string{"mp4", "jpg"} {
			name := filepath.Join(mediaDir, fmt.Sprintf("Recent %02d.%s", position, extension))
			if err := os.WriteFile(name, []byte(extension), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	response := httptest.NewRecorder()
	server.New(server.Config{MediaDir: mediaDir}).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	body := response.Body.String()
	start, end := strings.Index(body, "Recently added"), strings.Index(body, `<section class="destination-browser"`)
	if start < 0 || end < 0 {
		t.Fatalf("home has no recent shelf: %q", body)
	}
	recent := body[start:end]
	if strings.Count(recent, `fetchpriority="high"`) != 2 || strings.Count(recent, `loading="lazy"`) != 1 {
		t.Fatalf("recent artwork loading = %q", recent)
	}
}

func TestHomeRecentlyAddedUsesShowPosterAndCleanTitle(t *testing.T) {
	t.Parallel()
	mediaDir := t.TempDir()
	show := filepath.Join(mediaDir, "The Boys (2019) {imdb tt1190634}")
	season := filepath.Join(show, "Season 04")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(show, "poster.jpg"), []byte("poster"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(season, "The Boys - S04E01 - Department of Dirty Tricks.mkv"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.New(server.Config{MediaDir: mediaDir}).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	body := response.Body.String()
	if !strings.Contains(body, `<img class="poster" src="/art/`) || !strings.Contains(body, `>The Boys · S04E01 · Department of Dirty Tricks</h2>`) || strings.Contains(body, `{imdb`) {
		t.Fatalf("recent show card = %q", body)
	}
}

func TestHomeRecentlyAddedStacksEpisodesAndReusesArtwork(t *testing.T) { //nolint:cyclop // One populated-home scenario covers grouping and artwork reuse.
	t.Parallel()
	mediaDir := t.TempDir()
	season := filepath.Join(mediaDir, "Shared Poster", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(season, "Shared Poster - S01E01 - Pilot.mkv")
	second := filepath.Join(season, "Shared Poster - S01E02 - Next.mkv")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(strings.TrimSuffix(first, ".mkv")+"-thumb.jpg", []byte("poster"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(first, time.Unix(1, 0), time.Unix(1, 0)); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	body := response.Body.String()
	match := regexp.MustCompile(`class="card recent-card stacked" href="/show/([a-f0-9]+)"[^>]*>.*?src="/art/([a-f0-9]+)"`).FindStringSubmatch(body)
	if len(match) != 3 || match[1] == match[2] {
		t.Fatalf("recent stacked card does not reuse sibling artwork: %q", body)
	}
	if !strings.Contains(body, `>Shared Poster</h2>`) || !strings.Contains(body, `2 episodes stacked`) || strings.Contains(body, `Shared Poster · S01E02 · Next</h2>`) {
		t.Fatalf("recent stacked card has the wrong representation: %q", body)
	}
	art := httptest.NewRecorder()
	handler.ServeHTTP(art, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/art/"+match[2], nil))
	if art.Code != http.StatusOK || art.Body.String() != "poster" {
		t.Fatalf("art = %d %q", art.Code, art.Body.String())
	}
}

func TestHomeShowsRecentlyPlayedMedia(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Heat.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir()})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/progress/"+id, strings.NewReader("seconds=30"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	home = httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	body := home.Body.String()
	start := strings.Index(body, "Recently played")
	end := -1
	if start >= 0 {
		end = strings.Index(body[start:], "</section>")
	}
	if start < 0 || end < 0 {
		t.Fatalf("home = %q", body)
	}
	played := body[start : start+end]
	if !strings.Contains(played, "Recently played") || !strings.Contains(played, `href="/watch/`+id+`"`) || strings.Contains(played, "recent-card stacked") {
		t.Fatalf("recently played movie = %q", played)
	}
}

func TestHomeRecentlyPlayedStacksEpisodesByShow(t *testing.T) { //nolint:cyclop // One populated-home scenario covers playback grouping.
	t.Parallel()

	mediaDir := t.TempDir()
	season := filepath.Join(mediaDir, "Shared Poster", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Shared Poster - S01E01 - Pilot.mkv", "Shared Poster - S01E02 - Next.mkv"} {
		if err := os.WriteFile(filepath.Join(season, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir()})
	api := httptest.NewRecorder()
	handler.ServeHTTP(api, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
	ids := regexp.MustCompile(`"id":"([a-f0-9]+)"`).FindAllStringSubmatch(api.Body.String(), -1)
	if len(ids) != 2 {
		t.Fatalf("library ids = %q", api.Body.String())
	}
	for _, id := range ids {
		progress := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/progress/"+id[1], strings.NewReader("seconds=30"))
		progress.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		handler.ServeHTTP(httptest.NewRecorder(), progress)
	}
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	body := home.Body.String()
	start := strings.Index(body, "Recently played")
	end := strings.Index(body[start:], "</section>")
	if start < 0 || end < 0 {
		t.Fatalf("recently played shelf = %q", body)
	}
	played := body[start : start+end]
	if !strings.Contains(played, `class="card recent-card stacked" href="/show/`) || !strings.Contains(played, `aria-label="Shared Poster, 2 recently played episodes stacked; open show"`) || !strings.Contains(played, `2 episodes stacked`) || !strings.Contains(played, `<span class="recent-stack-count">2</span>`) || strings.Contains(played, `Shared Poster · S01E02 · Next</h2>`) {
		t.Fatalf("recently played stack = %q", played)
	}
}
