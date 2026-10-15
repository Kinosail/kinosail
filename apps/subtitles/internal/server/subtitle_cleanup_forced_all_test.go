package server_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestCleanupKeepsForcedInEveryLanguageButHidesUnselectedLanguages(t *testing.T) {
	media, tools := t.TempDir(), t.TempDir()
	writeTestFile(t, filepath.Join(media, "Film.mp4"), "video")
	for _, name := range []string{"Film.en.srt", "Film.es.srt", "Film.fr.srt", "Film.it.srt", "Film.en.forced.srt", "Film.fr.forced.srt", "Film.de.forced.vtt"} {
		writeTestFile(t, filepath.Join(media, name), name)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":1920,"height":1080},{"index":1,"codec_type":"audio","codec_name":"aac"}],"format":{"format_name":"mp4","duration":"120"}}'
`)
	data := t.TempDir()
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: data, CacheDir: t.TempDir(), FFprobe: ffprobe})
	firstSubtitleInventoryID(t, handler)
	preview := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitles/cleanup/preview", `{"enabled":true,"languages":["en","es"],"forced":"keep"}`)
	var plan struct {
		Digest string `json:"digest"`
		Files  []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if preview.Code != http.StatusOK || json.Unmarshal(preview.Body.Bytes(), &plan) != nil || len(plan.Digest) != 64 || len(plan.Files) != 2 {
		t.Fatalf("preview: %d %s", preview.Code, preview.Body.String())
	}
	for _, file := range plan.Files {
		if !strings.HasSuffix(file.Path, "Film.fr.srt") && !strings.HasSuffix(file.Path, "Film.it.srt") {
			t.Fatalf("preview would hide a kept track: %s", file.Path)
		}
	}
	conflict := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitles/cleanup", `{"enabled":true,"languages":["en","es"],"forced":"hide","digest":"`+plan.Digest+`"}`)
	if conflict.Code < http.StatusBadRequest {
		t.Fatalf("changed forced choice accepted: %d %s", conflict.Code, conflict.Body.String())
	}
	for _, name := range []string{"Film.fr.srt", "Film.fr.forced.srt"} {
		if _, err := os.Stat(filepath.Join(media, name)); err != nil {
			t.Fatalf("rejected cleanup changed %s: %v", name, err)
		}
	}
	confirmed := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitles/cleanup", `{"enabled":true,"languages":["en","es"],"forced":"keep","digest":"`+plan.Digest+`"}`)
	if confirmed.Code != http.StatusOK {
		t.Fatalf("confirm: %d %s", confirmed.Code, confirmed.Body.String())
	}
	for _, name := range []string{"Film.fr.srt", "Film.it.srt"} {
		if _, err := os.Stat(filepath.Join(media, name)); !os.IsNotExist(err) {
			t.Fatalf("unselected subtitle remains %s: %v", name, err)
		}
		content, err := os.ReadFile(filepath.Join(media, name+".hidden"))
		if err != nil || string(content) != name {
			t.Fatalf("hidden subtitle content %s: %q %v", name, content, err)
		}
	}
	for _, name := range []string{"Film.en.srt", "Film.es.srt", "Film.en.forced.srt", "Film.fr.forced.srt", "Film.de.forced.vtt"} {
		if _, err := os.Stat(filepath.Join(media, name)); err != nil {
			t.Fatalf("kept subtitle missing %s: %v", name, err)
		}
	}
}
