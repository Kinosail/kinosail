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

func TestShowEpisodeLedgerUsesNextEpisodeStillWithoutReplacingShowArtwork(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	season := filepath.Join(mediaDir, "Example Show", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(mediaDir, "Example Show", "poster.jpg"):         "show poster",
		filepath.Join(season, "Example Show S01E01 First.mkv"):        "first",
		filepath.Join(season, "Example Show S01E02 Second.mkv"):       "second",
		filepath.Join(season, "Example Show S01E02 Second-thumb.jpg"): "episode still",
		filepath.Join(season, "Example Show S01E02 Second.nfo"):       `<episodedetails><title>Second</title><plot>A focused episode plot.</plot></episodedetails>`,
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
	episodes := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindAllStringSubmatch(show.Body.String(), -1)
	if len(episodes) < 2 {
		t.Fatalf("episodes = %q", show.Body.String())
	}
	secondID := episodes[len(episodes)-1][1]
	watched := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/watched/"+episodes[0][1], strings.NewReader("watched=true"))
	watched.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), watched)
	show = httptest.NewRecorder()
	handler.ServeHTTP(show, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/"+showID, nil))

	assertShowEpisodeLedger(t, show.Body.String(), secondID)
	assertEpisodeArtworkRoutes(t, handler, showID, secondID)
}

func assertShowEpisodeLedger(t *testing.T, body, secondID string) {
	t.Helper()
	for _, expected := range []string{
		`class="episode-still" aria-hidden="true"`, `loading="lazy" width="160" height="90"`,
		`class="season-reel"`, `class="episode-preview"`, `class="episode-ledger"`,
		`detail-page show-detail`, `href="#seasons"`, `Jump to seasons`, `id="seasons"`, `class="season-index"`, `aria-label="Jump to season"`, `href="#season-1"`, `id="season-1"`,
		`class="episode-preview" href="/watch/` + secondID + `"`, `src="/art/` + secondID + `?variant=episode"`,
		`data-episode-art="/art/` + secondID + `?variant=episode"`, `>02</span>`, `>Second</strong>`,
		`<script defer src="/static/main.kinosail.bundle.js`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("show does not contain %q: %q", expected, body)
		}
	}
	if strings.Contains(body, `class="play"`) || strings.Contains(body, `>S01E02 · Second</strong>`) {
		t.Fatalf("show kept the repeated card treatment: %q", body)
	}
}

func assertEpisodeArtworkRoutes(t *testing.T, handler http.Handler, showID, secondID string) {
	t.Helper()
	api := httptest.NewRecorder()
	handler.ServeHTTP(api, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/shows/"+showID, nil))
	if !strings.Contains(api.Body.String(), `"artwork":"/art/`+secondID+`?variant=episode"`) {
		t.Fatalf("show API = %q", api.Body.String())
	}
	still := httptest.NewRecorder()
	handler.ServeHTTP(still, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/art/"+secondID+"?variant=episode", nil))
	if still.Code != http.StatusOK || still.Body.String() != "episode still" {
		t.Fatalf("episode art = %d %q", still.Code, still.Body.String())
	}
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/art/missing?variant=episode", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing episode art = %d %q", missing.Code, missing.Body.String())
	}
}
