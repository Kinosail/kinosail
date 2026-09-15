package servertest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// SubtitlePreferenceFixture binds a real application to its media tool paths.
type SubtitlePreferenceFixture struct {
	NewHandler func(media, data, ffprobe, ffmpeg string) http.Handler
}

// AlwaysOnSubtitlesRemainDirect verifies subtitle extraction without video transcoding.
func (fixture SubtitlePreferenceFixture) AlwaysOnSubtitlesRemainDirect(t *testing.T) { //nolint:cyclop // One lifecycle proves setting, direct media, and subtitle extraction.
	t.Parallel()
	media, data, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	WriteExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","profile":"High","level":40,"width":1920,"height":1080},{"index":1,"codec_type":"audio","codec_name":"aac","profile":"LC","disposition":{"default":1}},{"index":2,"codec_type":"subtitle","codec_name":"hdmv_pgs_subtitle","disposition":{"default":1},"tags":{"language":"eng"}},{"index":3,"codec_type":"subtitle","codec_name":"subrip","disposition":{"default":0},"tags":{"language":"eng"}}],"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2"}}'
`)
	arguments, ffmpeg := filepath.Join(tools, "arguments"), filepath.Join(tools, "ffmpeg")
	WriteExecutable(t, ffmpeg, fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> '%s'\nprintf 'WEBVTT\\n\\n00:00.000 --> 00:01.000\\nHello\\n'\n", arguments))
	handler := fixture.NewHandler(media, data, ffprobe, ffmpeg)
	save := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/playback", strings.NewReader("mode=automatic&subtitles=on"))
	save.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, save)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("save subtitles = %d %q", response.Code, response.Body.String())
	}

	handler = fixture.NewHandler(media, data, ffprobe, ffmpeg)
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/settings", nil))
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))

	if !strings.Contains(settings.Body.String(), `"subtitles":"on"`) {
		t.Fatalf("settings = %q", settings.Body.String())
	}
	if !strings.Contains(player.Body.String(), `src="/media/`+id+`?playbackSession=`) || !strings.Contains(player.Body.String(), `<track default kind="subtitles"`) || strings.Contains(player.Body.String(), `data-hls=`) {
		t.Fatalf("player did not keep separate subtitles on direct media: %q", player.Body.String())
	}
	playback := httptest.NewRecorder()
	handler.ServeHTTP(playback, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/"+id+"/playback", nil))
	if playback.Code != http.StatusOK || !strings.Contains(playback.Body.String(), `"mode":"direct"`) || !strings.Contains(playback.Body.String(), `"source":"/subtitle/`+id+`/embedded/3","default":true`) || strings.Contains(playback.Body.String(), `embedded/2`) {
		t.Fatalf("playback API = %d %q", playback.Code, playback.Body.String())
	}
	subtitle := httptest.NewRecorder()
	handler.ServeHTTP(subtitle, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitle/"+id+"/embedded/3", nil))
	used, err := os.ReadFile(arguments)
	if subtitle.Code != http.StatusOK || !strings.Contains(subtitle.Body.String(), "WEBVTT") || err != nil || !strings.Contains(string(used), "-map 0:3 -f webvtt pipe:1") {
		t.Fatalf("subtitle extraction = %d %q arguments=%q err=%v", subtitle.Code, subtitle.Body.String(), used, err)
	}
}
