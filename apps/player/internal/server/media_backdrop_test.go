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

func TestMovieUsesItsLandscapeArtworkAsTheBackdrop(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	for name, content := range map[string]string{
		"Home Alone (1990).mp4":        "movie",
		"Home Alone (1990)-poster.jpg": "poster",
		"Home Alone (1990)-fanart.jpg": "landscape",
	} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=movies", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]

	player, backdrop, api := httptest.NewRecorder(), httptest.NewRecorder(), httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	handler.ServeHTTP(backdrop, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/backdrop/"+id, nil))
	handler.ServeHTTP(api, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))

	if !strings.Contains(player.Body.String(), `class="player-page has-media-backdrop"`) || !strings.Contains(player.Body.String(), `class="media-backdrop" src="/backdrop/`+id+`"`) || strings.Contains(player.Body.String(), `style=`) {
		t.Fatalf("player has no title backdrop: %q", player.Body.String())
	}
	if !strings.Contains(player.Body.String(), `poster="/backdrop/`+id+`"`) {
		t.Fatal("video must use available landscape artwork before poster artwork")
	}
	if backdrop.Code != http.StatusOK || backdrop.Body.String() != "landscape" {
		t.Fatalf("backdrop = %d %q", backdrop.Code, backdrop.Body.String())
	}
	if !strings.Contains(api.Body.String(), `"backdrop":"/backdrop/`+id+`"`) {
		t.Fatalf("API has no backdrop: %q", api.Body.String())
	}
}

func TestShowUsesItsLandscapeArtworkAsTheBackdrop(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	season := filepath.Join(mediaDir, "Example Show", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(mediaDir, "Example Show", "poster.jpg"):                "poster",
		filepath.Join(mediaDir, "Example Show", "fanart.jpg"):                "landscape",
		filepath.Join(season, "Example Show S01E01 Example Episode One.mkv"): "episode",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=shows", nil))
	showID := regexp.MustCompile(`/show/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	artworkID := regexp.MustCompile(`/art/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]

	show, backdrop, api := httptest.NewRecorder(), httptest.NewRecorder(), httptest.NewRecorder()
	handler.ServeHTTP(show, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/"+showID, nil))
	handler.ServeHTTP(backdrop, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/backdrop/"+artworkID, nil))
	handler.ServeHTTP(api, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/shows/"+showID, nil))

	if !strings.Contains(show.Body.String(), `class="media-hero has-media-backdrop"`) || !strings.Contains(show.Body.String(), `class="media-backdrop" src="/backdrop/`+artworkID+`"`) || strings.Contains(show.Body.String(), `style=`) {
		t.Fatalf("show has no title backdrop: %q", show.Body.String())
	}
	if backdrop.Code != http.StatusOK || backdrop.Body.String() != "landscape" {
		t.Fatalf("backdrop = %d %q", backdrop.Code, backdrop.Body.String())
	}
	if !strings.Contains(api.Body.String(), `"backdrop":"/backdrop/`+artworkID+`"`) {
		t.Fatalf("API has no backdrop: %q", api.Body.String())
	}
}

func TestShowPosterFallbackKeepsThePosterSeparateFromBackdrop(t *testing.T) {
	t.Parallel()

	mediaDir := filepath.Join(t.TempDir(), "Example Show", "Season 01")
	if err := os.MkdirAll(mediaDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(filepath.Dir(mediaDir), "poster.jpg"):      "poster",
		filepath.Join(mediaDir, "Example Show S01E01 Pilot.mkv"): "episode",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: filepath.Dir(filepath.Dir(mediaDir))})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=shows", nil))
	showID := regexp.MustCompile(`/show/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]

	show := httptest.NewRecorder()
	handler.ServeHTTP(show, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/"+showID, nil))
	body := show.Body.String()
	if strings.Contains(body, `class="media-hero has-media-backdrop"`) || strings.Contains(body, `class="media-backdrop"`) {
		t.Fatalf("poster fallback was rendered as backdrop: %q", body)
	}
	if !strings.Contains(body, `class="hero-poster" src="/art/`) {
		t.Fatalf("poster fallback is missing poster artwork: %q", body)
	}
}

func TestBackdropFallsBackToPosterAndMissingArtworkStaysPlain(t *testing.T) { //nolint:cyclop,gocognit // One table covers the fallback and no-art branches end to end.
	t.Parallel()

	for _, test := range []struct {
		name, artwork, want string
	}{
		{"poster fallback", "poster", "poster"},
		{"missing artwork", "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			mediaDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(mediaDir, "Movie.mp4"), []byte("movie"), 0o600); err != nil {
				t.Fatal(err)
			}
			if test.artwork != "" {
				if err := os.WriteFile(filepath.Join(mediaDir, "Movie.jpg"), []byte(test.artwork), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			handler := server.New(server.Config{MediaDir: mediaDir})
			home := httptest.NewRecorder()
			handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=movies", nil))
			id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
			player, backdrop := httptest.NewRecorder(), httptest.NewRecorder()
			handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
			handler.ServeHTTP(backdrop, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/backdrop/"+id, nil))

			if test.want == "" {
				if strings.Contains(player.Body.String(), "has-media-backdrop") || backdrop.Code != http.StatusNotFound {
					t.Fatalf("missing artwork = page %q, backdrop %d", player.Body.String(), backdrop.Code)
				}
				return
			}
			if !strings.Contains(player.Body.String(), "has-media-backdrop") || backdrop.Code != http.StatusOK || backdrop.Body.String() != test.want {
				t.Fatalf("poster fallback = page %q, backdrop %d %q", player.Body.String(), backdrop.Code, backdrop.Body.String())
			}
		})
	}
}
