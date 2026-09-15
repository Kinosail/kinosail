package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestAudiobooksHaveDedicatedResumableChapterExperience(t *testing.T) {
	t.Parallel()

	media, tools := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "The Hobbit.m4b"), []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"audio","codec_name":"aac"}],"chapters":[{"start_time":"0","end_time":"60","tags":{"title":"An Unexpected Party"}},{"start_time":"60","end_time":"120","tags":{"title":"Roast Mutton"}}],"format":{"duration":"120","tags":{"title":"The Hobbit","artist":"J. R. R. Tolkien"}}}'
`)
	handler := server.New(server.Config{MediaDir: media, FFprobe: ffprobe})
	api := apiCall(t, handler, "", http.MethodGet, "/api/v1/library?view=audiobooks", nil)
	assertAPIBody(t, api, http.StatusOK, `"kind":"audiobook"`, "The Hobbit")
	home := apiCall(t, handler, "", http.MethodGet, "/?view=audiobooks", nil)
	assertAPIBody(t, home, http.StatusOK, "Audiobooks", "The Hobbit")
	if strings.Contains(home.Body.String(), "Nothing found.") {
		t.Fatalf("audiobook view rendered an empty state: %q", home.Body.String())
	}
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	player := apiCall(t, handler, "", http.MethodGet, "/watch/"+id, nil)
	assertAPIBody(t, player, http.StatusOK, "An Unexpected Party", "Roast Mutton", "Playback speed", "Sleep timer", `data-progress="/progress/`+id+`"`)
	script := apiCall(t, handler, "", http.MethodGet, "/static/player.js", nil)
	assertAPIBody(t, script, http.StatusOK, "[data-playback-rate]", "player.playbackRate", "[data-sleep-timer]", "player.pause()")
}
