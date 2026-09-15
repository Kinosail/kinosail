package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestEmbeddedMusicTagsOrganizeLibraryThroughAPIAndWeb(t *testing.T) {
	t.Parallel()

	media, tools := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "unknown.mp3"), []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"format":{"tags":{"title":"An Eagle in Your Mind","artist":"Boards of Canada","album_artist":"Boards of Canada","album":"Music Has the Right to Children","genre":"Electronic","track":"2/18","disc":"1/1"}}}'
`)
	handler := server.New(server.Config{MediaDir: media, FFprobe: ffprobe})
	api := apiCall(t, handler, "", http.MethodGet, "/api/v1/library", nil)
	assertAPIBody(t, api, http.StatusOK, "An Eagle in Your Mind", "Boards of Canada", "Music Has the Right to Children", "Electronic")
	home := apiCall(t, handler, "", http.MethodGet, "/?view=music", nil)
	assertAPIBody(t, home, http.StatusOK, "Boards of Canada", "Music Has the Right to Children")
}
