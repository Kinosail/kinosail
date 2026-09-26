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

func TestHomeFollowsMobileWatchSectionOrderAndViewerProgress(t *testing.T) {
	t.Parallel()
	media := t.TempDir()
	for _, name := range []string{"Fresh Movie.mp4", "Watched Movie.mp4", "Record.mp3", "Story.m4b", "Book.pdf"} {
		if err := os.WriteFile(filepath.Join(media, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(media, "Fresh Movie.nfo"), []byte("<movie><title>Fresh Movie</title><genre>Drama</genre></movie>"), 0o600); err != nil {
		t.Fatal(err)
	}
	season := filepath.Join(media, "Fresh Show", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(season, "Fresh Show - S01E01 - Pilot.mkv"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: media, DataDir: t.TempDir()})
	get := func(path string) string {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s: %d", path, response.Code)
		}
		return response.Body.String()
	}
	match := regexp.MustCompile(`href="/item/([a-f0-9]+)"`).FindStringSubmatch(get("/?q=Watched+Movie"))
	if len(match) != 2 {
		t.Fatal("watched movie was not found")
	}
	mark := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/watched/"+match[1], strings.NewReader("watched=true"))
	mark.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), mark)
	body := get("/")
	order := []string{"Recently added movies", "Recently added TV shows", "Unwatched TV shows", "Unwatched movies", "Movie genres", "Recently added music", "Recently added audiobooks"}
	previous := -1
	for _, title := range order {
		position := strings.Index(body, ">"+title+"</h2>")
		if position <= previous {
			t.Fatalf("home section %q is missing or out of order", title)
		}
		previous = position
	}
	section := func(name string) string {
		expression := regexp.MustCompile(`(?s)<section class="home-shelf" data-home-shelf="` + name + `".*?</section>`)
		return expression.FindString(body)
	}
	if !strings.Contains(section("recent-movies"), "Watched Movie") || strings.Contains(section("unwatched-movies"), "Watched Movie") || !strings.Contains(section("unwatched-movies"), "Fresh Movie") {
		t.Fatal("unwatched movies did not follow saved viewer progress")
	}
	if !strings.Contains(section("recent-shows"), `href="/watch/`) || !strings.Contains(section("unwatched-shows"), "Fresh Show") || strings.Contains(section("recent-movies"), "Fresh Show") {
		t.Fatal("TV shows were not separated from movies")
	}
	if strings.Contains(section("recent-music"), "Story") || strings.Contains(section("recent-audiobooks"), "Record") || !strings.Contains(section("recent-other"), "Book") {
		t.Fatal("non-video home media is missing or misclassified")
	}
}
