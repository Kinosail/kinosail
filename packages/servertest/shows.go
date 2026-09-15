package servertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ViewerCanBrowseAShowsEpisodes verifies browsing and watched-state projection.
func ViewerCanBrowseAShowsEpisodes(t *testing.T, newHandler func(*testing.T, string) http.Handler, expected ...string) {
	t.Helper()
	mediaDir := t.TempDir()
	season := filepath.Join(mediaDir, "Severance", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(season, "Severance.S01E01.Good.News.mkv"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := newHandler(t, mediaDir)
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=shows", nil))
	match := regexp.MustCompile(`/show/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())
	if len(match) != 2 {
		t.Fatalf("home has no show link: %q", home.Body.String())
	}
	show := httptest.NewRecorder()
	handler.ServeHTTP(show, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/"+match[1], nil))
	episodeID := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(show.Body.String())[1]
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/watched/"+episodeID, strings.NewReader("watched=true"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	show = httptest.NewRecorder()
	handler.ServeHTTP(show, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/"+match[1], nil))
	for _, value := range expected {
		if !strings.Contains(show.Body.String(), value) {
			t.Fatalf("show lacks %q: %q", value, show.Body.String())
		}
	}
}
