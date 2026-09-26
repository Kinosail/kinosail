package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestEpisodeStillServesOneCachedFramePerVisibleEpisode(t *testing.T) {
	media, cache := t.TempDir(), t.TempDir()
	season := filepath.Join(media, "Series", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Series S01E01 First.mp4", "Series S01E02 Second.mp4"} {
		if err := os.WriteFile(filepath.Join(season, name), []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	calls := filepath.Join(t.TempDir(), "calls")
	ffmpeg := filepath.Join(t.TempDir(), "ffmpeg")
	script := "#!/bin/sh\nframe=unknown\nfor arg do\n  case \"$arg\" in\n    *S01E01*) frame=first;;\n    *S01E02*) frame=second;;\n  esac\n  target=$arg\ndone\nprintf '%s' \"$frame\" > \"$target\"\nprintf x >> \"" + calls + "\"\n"
	if err := os.WriteFile(ffmpeg, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: "/bin/false"})
	show := httptest.NewRecorder()
	handler.ServeHTTP(show, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/shows/"+library.ShowID("Series"), nil))
	var catalog struct {
		Episodes []struct {
			ID, Artwork string
			Episode     int
		} `json:"episodes"`
	}
	if err := json.Unmarshal(show.Body.Bytes(), &catalog); err != nil || show.Code != http.StatusOK || len(catalog.Episodes) != 2 {
		t.Fatalf("show = %d, %v, %q", show.Code, err, show.Body.String())
	}
	for _, path := range []string{"/episode-art/", "/episode-art/missing", "/episode-art/invalid!", "/episode-art/" + strings.Repeat("a", 129), "/episode-art/" + catalog.Episodes[0].ID + "?variant=show", "/episode-art/" + catalog.Episodes[0].ID + "?variant=episode&variant=episode", "/episode-art/" + catalog.Episodes[0].ID + "?" + strings.Repeat("x", 16_385)} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s = %d", path, response.Code)
		}
	}
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Fatalf("rejected requests started frame generation: %v", err)
	}
	for _, episode := range catalog.Episodes {
		if episode.Artwork != "/episode-art/"+episode.ID {
			t.Fatalf("episode artwork = %q", episode.Artwork)
		}
		for attempt := 0; attempt < 2; attempt++ {
			still := httptest.NewRecorder()
			handler.ServeHTTP(still, httptest.NewRequestWithContext(t.Context(), http.MethodGet, episode.Artwork, nil))
			if still.Code != http.StatusOK || still.Header().Get("Content-Type") != "image/jpeg" || still.Body.String() != []string{"first", "second"}[episode.Episode-1] {
				t.Fatalf("episode %s still = %d %q", episode.ID, still.Code, still.Body.String())
			}
		}
	}
	data, err := os.ReadFile(calls)
	if err != nil || string(data) != "xx" {
		t.Fatalf("frame generation calls = %q, %v", data, err)
	}
	if matches, _ := filepath.Glob(filepath.Join(cache, "episode-stills", "*")); len(matches) != 2 {
		t.Fatalf("cached stills = %v", matches)
	}
}
