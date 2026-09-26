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

func TestCleanupKeepsForcedInEveryLanguageAndOffersThemForPlayback(t *testing.T) { //nolint:gocognit,funlen,cyclop // One public journey proves the preview, deletion, saved choice, and playback options agree.
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
	id := firstSubtitleInventoryID(t, handler)
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
			t.Fatalf("preview would remove a kept track: %s", file.Path)
		}
	}
	conflict := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitles/cleanup", `{"enabled":true,"languages":["en","es"],"forced":"delete","digest":"`+plan.Digest+`"}`)
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
	}
	for _, name := range []string{"Film.en.srt", "Film.es.srt", "Film.en.forced.srt", "Film.fr.forced.srt", "Film.de.forced.vtt"} {
		if _, err := os.Stat(filepath.Join(media, name)); err != nil {
			t.Fatalf("kept subtitle missing %s: %v", name, err)
		}
	}
	page := requestApp(t, handler, http.MethodGet, "/watch/"+id, "")
	api := requestApp(t, handler, http.MethodGet, "/api/v1/items/"+id+"/playback", "")
	var playback struct {
		Subtitles []struct {
			Language string `json:"language"`
			Role     string `json:"role"`
		} `json:"subtitles"`
	}
	if page.Code != http.StatusOK || strings.Count(page.Body.String(), "<track ") != 5 || api.Code != http.StatusOK || json.Unmarshal(api.Body.Bytes(), &playback) != nil || len(playback.Subtitles) != 5 {
		t.Fatalf("playback choices: page=%d API=%d %s", page.Code, api.Code, api.Body.String())
	}
	for _, want := range []string{"en/translation", "es/translation", "en/forced", "fr/forced", "de/forced"} {
		found := false
		for _, track := range playback.Subtitles {
			found = found || track.Language+"/"+track.Role == want
		}
		if !found {
			t.Errorf("missing playback option %s: %+v", want, playback.Subtitles)
		}
	}
	restarted := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: data, CacheDir: t.TempDir(), FFprobe: ffprobe})
	restartedID := firstSubtitleInventoryID(t, restarted)
	reopened := requestApp(t, restarted, http.MethodGet, "/watch/"+restartedID, "")
	if reopened.Code != http.StatusOK || strings.Count(reopened.Body.String(), "<track ") != 5 {
		t.Fatalf("forced picker choice was not saved: %d tracks=%d", reopened.Code, strings.Count(reopened.Body.String(), "<track "))
	}
}
