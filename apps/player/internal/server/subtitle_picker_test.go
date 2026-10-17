package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

type pickerTrack struct {
	Label, Source string
	Default       bool
}

const pickerProbe = `#!/bin/sh
printf '%%s' '{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":1920,"height":1080},{"index":1,"codec_type":"audio","codec_name":"aac","disposition":{"default":1},"tags":{"language":"%s"}},{"index":2,"codec_type":"subtitle","codec_name":"subrip","disposition":{"default":1},"tags":{"language":"eng","title":"English"}},{"index":3,"codec_type":"subtitle","codec_name":"subrip","disposition":{"hearing_impaired":1},"tags":{"language":"eng","title":"English (SDH)"}},{"index":4,"codec_type":"subtitle","codec_name":"subrip","disposition":{"forced":1},"tags":{"language":"eng","title":"Signs & Songs"}},{"index":5,"codec_type":"subtitle","codec_name":"ass","tags":{"language":"eng","title":"Dialogue"}},{"index":6,"codec_type":"subtitle","codec_name":"subrip","disposition":{"comment":1},"tags":{"language":"eng","title":"Director'"'"'s Commentary"}},{"index":7,"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"spa"}}],"format":{"format_name":"matroska","duration":"60"}}'
`

func TestSubtitlePickerIsReadableGroupedAndDefaultsByAudioLanguage(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		audio, mode string
		selected    int
	}{
		{"fra", "", 0},
		{"eng", "", 2},
		{"und", "", 2},
		{"eng", "on", 0},
		{"fra", "off", -1},
	} {
		t.Run(test.audio+"-"+test.mode, func(t *testing.T) {
			t.Parallel()
			handler, id := pickerServer(t, test.audio)
			if test.mode != "" {
				request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/playback", strings.NewReader("mode=automatic&subtitles="+test.mode))
				request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				saved := httptest.NewRecorder()
				handler.ServeHTTP(saved, request)
				if saved.Code != http.StatusSeeOther {
					t.Fatalf("save subtitles=%s: %d %s", test.mode, saved.Code, saved.Body.String())
				}
			}
			tracks := pickerTracks(t, handler, id)
			assertPickerTracks(t, id, tracks, test.selected)
			assertPickerWeb(t, handler, id, test.selected)
		})
	}
}

func pickerServer(t *testing.T, audio string) (http.Handler, string) {
	t.Helper()
	media, data, tools := t.TempDir(), t.TempDir(), t.TempDir()
	for name, contents := range map[string]string{"Film.mkv": "video", "Film.en.srt": "1\n00:00:01,000 --> 00:00:02,000\nHello\n"} {
		if err := os.WriteFile(filepath.Join(media, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	servertest.WriteExecutable(t, ffprobe, fmt.Sprintf(pickerProbe, audio))
	handler := server.New(server.Config{MediaDir: media, DataDir: data, FFprobe: ffprobe})
	return handler, firstWebItemID(t, handler)
}

func pickerTracks(t *testing.T, handler http.Handler, id string) []pickerTrack {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/"+id+"/playback", nil))
	var playback struct{ Subtitles []pickerTrack }
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &playback) != nil || strings.Contains(response.Body.String(), `"group"`) || strings.Contains(response.Body.String(), `"detail"`) {
		t.Fatalf("playback = %d %s", response.Code, response.Body.String())
	}
	return playback.Subtitles
}

func assertPickerTracks(t *testing.T, id string, tracks []pickerTrack, selected int) {
	t.Helper()
	want := []pickerTrack{
		{Label: "English", Source: "/subtitle/" + id + "/0"},
		{Label: "English [CC]", Source: "/subtitle/" + id + "/embedded/3"},
		{Label: "English · Foreign dialogue only · Signs & Songs", Source: "/subtitle/" + id + "/embedded/4"},
		{Label: "English · Director's Commentary", Source: "/subtitle/" + id + "/embedded/6"},
		{Label: "Spanish", Source: "/subtitle/" + id + "/embedded/7"},
		{Label: "English · Built-in", Source: "/subtitle/" + id + "/embedded/2"},
		{Label: "English · Dialogue · Built-in", Source: "/subtitle/" + id + "/embedded/5"},
	}
	if len(tracks) != len(want) {
		t.Fatalf("tracks = %+v", tracks)
	}
	for index := range want {
		want[index].Default = index == selected
		if tracks[index] != want[index] {
			t.Errorf("track %d = %+v, want %+v", index, tracks[index], want[index])
		}
	}
}

func assertPickerWeb(t *testing.T, handler http.Handler, id string, selected int) {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	body := response.Body.String()
	picker := regexp.MustCompile(`<select data-subtitles[^>]*>(.*?)</select>`).FindStringSubmatch(body)
	if response.Code != http.StatusOK || len(picker) != 2 {
		t.Fatalf("watch page = %d", response.Code)
	}
	more := regexp.MustCompile(`<optgroup label="More tracks">(.*?)</optgroup>`).FindStringSubmatch(picker[1])
	if len(more) != 2 || strings.Count(more[1], "<option") != 2 || !strings.Contains(more[1], `value="5"`) || !strings.Contains(more[1], `value="6"`) || !strings.Contains(picker[1], `value="1" title="Includes sound effects and speaker names for viewers who are deaf or hard of hearing."`) {
		t.Fatalf("More tracks group = %q", picker[1])
	}
	if strings.Count(picker[1], " selected") != map[bool]int{true: 1, false: 0}[selected >= 0] || selected >= 0 && !strings.Contains(picker[1], fmt.Sprintf(`value="%d" selected`, selected)) {
		t.Fatalf("selected subtitle in picker = %q, want %d", picker[1], selected)
	}
	if strings.Count(body, "<track default ") != map[bool]int{true: 1, false: 0}[selected >= 0] {
		t.Fatalf("default text tracks in %q", regexp.MustCompile(`<track[^>]*>`).FindAllString(body, -1))
	}
}
