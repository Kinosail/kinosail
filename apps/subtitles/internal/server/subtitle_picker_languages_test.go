package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestPlaybackPickerShowsOnlySelectedSubtitleLanguages(t *testing.T) { //nolint:gocognit,cyclop,funlen // One public journey covers default off, cleanup opt-in, choices, invalid toggles, and reset.
	media, tools := t.TempDir(), t.TempDir()
	writeTestFile(t, filepath.Join(media, "Film.mp4"), "video")
	for _, name := range []string{"Film.en.srt", "Film.es.srt", "Film.fr.srt", "Film.de.srt"} {
		writeTestFile(t, filepath.Join(media, name), name)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":1920,"height":1080},{"index":1,"codec_type":"audio","codec_name":"aac"},{"index":2,"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"eng"}},{"index":3,"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"spa"}},{"index":4,"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"fra"}}],"format":{"format_name":"mp4","duration":"120"}}'
`)
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), FFprobe: ffprobe})
	id := firstSubtitleInventoryID(t, handler)
	before := requestApp(t, handler, http.MethodGet, "/watch/"+id, "")
	if before.Code != http.StatusOK || strings.Count(before.Body.String(), "<track ") != 7 {
		t.Fatalf("default picker should remain unrestricted: %d %s", before.Code, before.Body.String())
	}
	preview := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitles/cleanup/preview", `{"enabled":true,"languages":["en","es","fr","de"],"forced":"keep"}`)
	var plan struct {
		Digest string `json:"digest"`
	}
	if preview.Code != http.StatusOK || json.Unmarshal(preview.Body.Bytes(), &plan) != nil || len(plan.Digest) != 64 {
		t.Fatalf("cleanup preview: %d %s", preview.Code, preview.Body.String())
	}
	confirmed := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitles/cleanup", `{"enabled":true,"languages":["en","es","fr","de"],"forced":"keep","digest":"`+plan.Digest+`"}`)
	if confirmed.Code != http.StatusOK {
		t.Fatalf("cleanup confirmation: %d %s", confirmed.Code, confirmed.Body.String())
	}
	for _, test := range []struct {
		languages string
		want      []string
	}{
		{`["en","fr"]`, []string{"/embedded/2", "/embedded/4"}},
		{`["fr"]`, []string{"/embedded/4"}},
		{`["de"]`, []string{"/0"}},
		{`["it"]`, nil},
	} {
		response := requestJSON(t, handler, http.MethodPut, "/api/v1/settings/subtitles", `{"languages":`+test.languages+`}`)
		if response.Code != http.StatusOK {
			t.Fatalf("save %s: %d %s", test.languages, response.Code, response.Body.String())
		}
		page := requestApp(t, handler, http.MethodGet, "/watch/"+id, "")
		match := regexp.MustCompile(`(?s)<select data-subtitles[^>]*>(.*?)</select>`).FindStringSubmatch(page.Body.String())
		if page.Code != http.StatusOK || len(match) != 2 || !strings.Contains(match[1], `value="off"`) {
			t.Fatalf("picker for %s: %d %s", test.languages, page.Code, page.Body.String())
		}
		api := requestApp(t, handler, http.MethodGet, "/api/v1/items/"+id+"/playback", "")
		var result struct {
			Subtitles []struct {
				Source string `json:"source"`
			} `json:"subtitles"`
		}
		if api.Code != http.StatusOK || json.Unmarshal(api.Body.Bytes(), &result) != nil || len(result.Subtitles) != len(test.want) {
			t.Fatalf("API tracks for %s: %d %s", test.languages, api.Code, api.Body.String())
		}
		if count := strings.Count(match[1], "<option"); count != len(test.want)+1 {
			t.Errorf("picker for %s has %d options, want %d: %s", test.languages, count, len(test.want)+1, match[1])
		}
		for index, suffix := range test.want {
			if !strings.HasSuffix(result.Subtitles[index].Source, suffix) {
				t.Errorf("API track %d for %s = %q, want %s", index, test.languages, result.Subtitles[index].Source, suffix)
			}
		}
	}
	for _, body := range []string{"", "limited=unknown", "limited=on&limited=off", "limited=on&extra=1", "limited=" + strings.Repeat("x", 4097)} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles/picker", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Errorf("invalid picker setting %q = %d", body, response.Code)
		}
	}
	limited := requestApp(t, handler, http.MethodGet, "/watch/"+id, "")
	if strings.Count(limited.Body.String(), "<track ") != 0 {
		t.Fatal("invalid picker settings changed the saved limit")
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/subtitles/picker", strings.NewReader("limited=off"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	after := requestApp(t, handler, http.MethodGet, "/watch/"+id, "")
	if response.Code != http.StatusSeeOther || strings.Count(after.Body.String(), "<track ") != 7 {
		t.Fatalf("turning off picker limit: %d, tracks=%d", response.Code, strings.Count(after.Body.String(), "<track "))
	}
}
