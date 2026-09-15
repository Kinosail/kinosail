package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestAlbumPlaybackExposesGaplessQueueThroughAPIAndWeb(t *testing.T) {
	t.Parallel()

	media := t.TempDir()
	for index, title := range []string{"First", "Second"} {
		base := filepath.Join(media, title)
		if err := os.WriteFile(base+".m4a", []byte(title), 0o600); err != nil {
			t.Fatal(err)
		}
		nfo := `<song><title>` + title + `</title><artist>Artist</artist><album>Album</album><track>` + string(rune('1'+index)) + `</track></song>`
		if err := os.WriteFile(base+".nfo", []byte(nfo), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: media})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=music", nil))
	albumID := regexp.MustCompile(`/album/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	album := apiCall(t, handler, "", http.MethodGet, "/api/v1/albums/"+albumID, nil)
	var albumData struct {
		Tracks []struct {
			ID string `json:"id"`
		} `json:"tracks"`
	}
	mustJSON(t, album, &albumData)
	firstID := albumData.Tracks[0].ID
	queue := apiCall(t, handler, "", http.MethodGet, "/api/v1/audio/"+firstID+"/queue", nil)
	assertAPIBody(t, queue, http.StatusOK, "First", "Second", "/media/")
	player := apiCall(t, handler, "", http.MethodGet, "/watch/"+firstID, nil)
	assertAPIBody(t, player, http.StatusOK, `data-queue="/api/v1/audio/`+firstID+`/queue"`)
	script := apiCall(t, handler, "", http.MethodGet, "/static/player.js", nil)
	if !strings.Contains(script.Body.String(), "advanceQueue") {
		t.Fatalf("player script does not advance a preloaded audio queue: %q", script.Body.String())
	}
}
