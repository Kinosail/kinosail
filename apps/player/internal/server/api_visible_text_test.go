package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"unicode"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestAppleClientsReceiveOnlyVisibleText(t *testing.T) {
	t.Parallel()
	media, tools := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film (2020).mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	nfo := "<movie><title>Film\u200b Night</title><plot>A soft\u00adhyphen\u200e plot\twith a tab.\nSecond line.</plot></movie>"
	if err := os.WriteFile(filepath.Join(media, "Film (2020).nfo"), []byte(nfo), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\"},{\"codec_type\":\"audio\",\"codec_name\":\"dts\",\"channels\":6}],\"chapters\":[{\"start_time\":\"0\",\"end_time\":\"60\",\"tags\":{\"title\":\"Cap\u00c3\u00adtulo 01\"}},{\"start_time\":\"60\",\"end_time\":\"120\",\"tags\":{\"title\":\"\u200fIntro\u200d\"}}],\"format\":{\"format_name\":\"matroska,webm\",\"duration\":\"120\"}}'\n")
	handler := server.New(server.Config{MediaDir: media, FFprobe: ffprobe})
	library := httptest.NewRecorder()
	handler.ServeHTTP(library, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
	var catalog struct {
		Items []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Plot  string `json:"plot"`
		} `json:"items"`
	}
	if library.Code != http.StatusOK || json.Unmarshal(library.Body.Bytes(), &catalog) != nil || len(catalog.Items) != 1 {
		t.Fatalf("library = %d %q", library.Code, library.Body.String())
	}
	if item := catalog.Items[0]; item.Title != "Film Night" || item.Plot != "A softhyphen plot with a tab.\nSecond line." {
		t.Fatalf("library text = %q %q", item.Title, item.Plot)
	}
	playback := httptest.NewRecorder()
	handler.ServeHTTP(playback, httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/api/v1/items/"+catalog.Items[0].ID+"/playback?videoCodecs=h264,hevc&audioCodecs=aac,mp3,ac3,eac3&hdrFormats=sdr&maxAudioChannels=8", nil))
	var answer struct {
		Chapters []struct {
			Title string `json:"title"`
		} `json:"chapters"`
	}
	if playback.Code != http.StatusOK || json.Unmarshal(playback.Body.Bytes(), &answer) != nil || len(answer.Chapters) != 2 {
		t.Fatalf("playback = %d %q", playback.Code, playback.Body.String())
	}
	if answer.Chapters[0].Title != "Cap\u00edtulo 01" || answer.Chapters[1].Title != "Intro" {
		t.Fatalf("chapter titles = %q %q", answer.Chapters[0].Title, answer.Chapters[1].Title)
	}
	for name, body := range map[string][]byte{"library": library.Body.Bytes(), "playback": playback.Body.Bytes()} {
		var value any
		_ = json.Unmarshal(body, &value)
		assertVisibleText(t, name, value)
	}
}

func assertVisibleText(t *testing.T, path string, value any) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			assertVisibleText(t, path+"."+key, child)
		}
	case []any:
		for _, child := range typed {
			assertVisibleText(t, path+"[]", child)
		}
	case string:
		for _, character := range typed {
			if character != '\n' && character != '\r' && (unicode.IsControl(character) || unicode.In(character, unicode.Cf)) {
				t.Fatalf("%s contains U+%04X in %q", path, character, typed)
			}
		}
	}
}
