package server

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

// These exports exercise the real show template in the browser layout regression.
func TestWriteShowActionFixtures(t *testing.T) {
	t.Parallel()
	dir := os.Getenv("KINOSAIL_UI_FIXTURE_DIR")
	if dir == "" {
		t.Skip("KINOSAIL_UI_FIXTURE_DIR is not set")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil { //nolint:gosec // Explicit test artifact directory.
		t.Fatal(err)
	}
	for _, state := range []string{"long", "unbroken", "watched", "empty"} {
		episode := episodeData{Item: library.Item{ID: "episode-1", Season: 1, Episode: 1}, DisplayTitle: "Take My Siblings Please / The Mindy 500 / Morning Malaise", Next: true}
		if state == "unbroken" {
			episode.DisplayTitle = strings.Repeat("LongEpisodeTitle", 20)
		}
		if state == "watched" {
			episode.Next, episode.Watched = false, true
		}
		data := showPageData{Show: library.Show{Title: "Animaniacs", Year: "1993", Plot: "The two Warner Brothers Yakko and Wakko and their Warner sister Dot run wild, causing chaos everywhere!"}, Next: &episode, EpisodeCount: 1, SeasonGroups: []seasonPageData{{Number: 1, Episodes: []episodeData{episode}, Preview: &episode}}}
		if state == "empty" {
			data.Next, data.SeasonGroups, data.EpisodeCount = nil, nil, 0
		}
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), "GET", "https://kinosail.test/show/example", nil)
		if err := showView.Execute(response, request, data); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "show-action-"+state+".html"), response.Body.Bytes(), 0o600); err != nil { //nolint:gosec // Fixed fixture names and explicit artifact directory.
			t.Fatal(err)
		}
	}
}
