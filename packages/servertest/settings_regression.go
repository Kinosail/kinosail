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

func (suite SettingsRegression) OwnerCanPreferHighQualityTranscoding(t *testing.T) {
	t.Parallel()

	mediaDir, dataDir, cacheDir := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Brazil.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	arguments := filepath.Join(t.TempDir(), "arguments")
	ffmpeg := filepath.Join(t.TempDir(), "ffmpeg")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> '%s'\n", arguments) + suite.PlayableHLS()
	//nolint:gosec // G306: the fake FFmpeg adapter must be executable.
	if err := os.WriteFile(ffmpeg, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(t.TempDir(), "ffprobe")
	WriteExecutable(t, probe, `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"video","codec_name":"hevc","width":1920,"height":1080},{"codec_type":"audio","codec_name":"aac","index":1}],"format":{"duration":"120"}}'
`)
	handler := suite.New(SettingsFixture{MediaDir: mediaDir, DataDir: dataDir, CacheDir: cacheDir, FFmpeg: ffmpeg, FFprobe: probe})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	playlist := httptest.NewRecorder()
	handler.ServeHTTP(playlist, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+id+"/index.m3u8", nil))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/transcoder", strings.NewReader("quality=quality"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	playlist = httptest.NewRecorder()
	handler.ServeHTTP(playlist, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+id+"/index.m3u8", nil))
	used, err := os.ReadFile(arguments)

	if err != nil || response.Code != http.StatusSeeOther || playlist.Code != http.StatusOK ||
		!strings.Contains(string(used), "-preset veryfast") || !strings.Contains(string(used), "-preset medium") || !strings.Contains(string(used), "-crf 19") {
		t.Fatalf("save = %d, playlist = %d, arguments = %q, error = %v", response.Code, playlist.Code, used, err)
	}
}

func (suite SettingsRegression) TranscoderDefaultsToRecommendedAutomaticSelections(t *testing.T) {
	t.Parallel()

	handler := suite.New(SettingsFixture{DataDir: t.TempDir()})
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))

	for _, expected := range []string{
		`name="quality" value="automatic" checked`,
		`<option value="auto" selected>Automatic per device (Recommended)`,
		`Automatic checks each playback device only when Kinosail must convert video. It prefers H.264 and uses only encoding paths that passed a local HLS smoke check. A manual choice forces that format for every playback device.`,
		`Automatic will use:</strong> Processor`,
	} {
		if !strings.Contains(settings.Body.String(), expected) {
			t.Fatalf("settings lacks %q: %q", expected, settings.Body.String())
		}
	}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/transcoder", strings.NewReader("quality=automatic&accelerator=none"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	settings = httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	if !strings.Contains(settings.Body.String(), `<option value="none" selected>Processor`) {
		t.Fatalf("explicit software selection was not preserved: %q", settings.Body.String())
	}
}

func (suite SettingsRegression) PlaybackSettingsExplainTheDirectFirstDefault(t *testing.T) {
	t.Parallel()
	handler := suite.New(SettingsFixture{DataDir: t.TempDir()})
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	for _, expected := range []string{"Direct First starts the original file on every connection.", "Direct First (Recommended)", "Direct Play only", "Compatibility first", "Video transcoding always asks first."} {
		if !strings.Contains(settings.Body.String(), expected) {
			t.Fatalf("settings lack %q: %q", expected, settings.Body.String())
		}
	}
}

func (suite SettingsRegression) SettingsDoNotExposeRetiredCloudBroker(t *testing.T) {
	t.Parallel()

	handler := suite.New(SettingsFixture{DataDir: t.TempDir()})
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	if settings.Code != http.StatusOK {
		t.Fatalf("settings status = %d", settings.Code)
	}
	for _, retired := range []string{"Kinosail Cloud", "Connection Broker", "Viewer Grant", "Cloud Connect"} {
		if strings.Contains(settings.Body.String(), retired) {
			t.Fatalf("settings still expose %q", retired)
		}
	}
	for _, path := range []string{"/settings/remote/grants", "/settings/remote/grants/revoke", "/api/v1/remote-access/grants", "/api/v1/remote-access/grants/grant-id"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("retired route %s status = %d", path, response.Code)
		}
	}
}

func (suite SettingsRegression) OwnerCanChooseCompatiblePlaybackByDefault(t *testing.T) {
	t.Parallel()

	mediaDir, dataDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Heat.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := suite.New(SettingsFixture{MediaDir: mediaDir, DataDir: dataDir})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/playback", strings.NewReader("mode=compatible"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	handler = suite.New(SettingsFixture{MediaDir: mediaDir, DataDir: dataDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))

	if response.Code != http.StatusSeeOther || !strings.Contains(player.Body.String(), `/hls/`+id+`/p/t-a0-s0-none-t0-b0/index.m3u8`) ||
		!strings.Contains(settings.Body.String(), `name="mode" value="compatible" checked`) {
		t.Fatalf("save = %d, player = %q, settings = %q", response.Code, player.Body.String(), settings.Body.String())
	}
}

func (suite SettingsRegression) OwnerCanAutoplayTheNextEpisode(t *testing.T) {
	t.Parallel()

	mediaDir, dataDir := t.TempDir(), t.TempDir()
	season := filepath.Join(mediaDir, "Severance", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Severance.S01E01.Good.News.mkv", "Severance.S01E02.Half.Loop.mkv"} {
		if err := os.WriteFile(filepath.Join(season, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := suite.New(SettingsFixture{MediaDir: mediaDir, DataDir: dataDir})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/playback", strings.NewReader("mode=automatic&autoplay=true"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	handler = suite.New(SettingsFixture{MediaDir: mediaDir, DataDir: dataDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=shows", nil))
	showID := regexp.MustCompile(`/show/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	show := httptest.NewRecorder()
	handler.ServeHTTP(show, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/show/"+showID, nil))
	episodes := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindAllStringSubmatch(show.Body.String(), -1)
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+episodes[0][1], nil))
	script := httptest.NewRecorder()
	handler.ServeHTTP(script, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/player.js", nil))

	if !strings.Contains(player.Body.String(), `data-next="/watch/`+episodes[len(episodes)-1][1]+`"`) || !strings.Contains(script.Body.String(), "location.assign") {
		t.Fatalf("player = %q, script = %q", player.Body.String(), script.Body.String())
	}
}

func (suite SettingsRegression) OwnerCanDisableSubtitlesByDefault(t *testing.T) {
	t.Parallel()

	mediaDir, dataDir := t.TempDir(), t.TempDir()
	for name, content := range map[string]string{"Moon.mp4": "video", "Moon.vtt": "WEBVTT"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := suite.New(SettingsFixture{MediaDir: mediaDir, DataDir: dataDir})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/playback", strings.NewReader("mode=automatic&subtitles=off"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	handler = suite.New(SettingsFixture{MediaDir: mediaDir, DataDir: dataDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))

	if !strings.Contains(player.Body.String(), `<track kind="subtitles"`) || strings.Contains(player.Body.String(), `<track default`) || !strings.Contains(settings.Body.String(), `name="subtitles" value="off" checked`) {
		t.Fatalf("player = %q, settings = %q", player.Body.String(), settings.Body.String())
	}
}

func (suite SettingsRegression) PlaybackModeRejectsInvalidValue(t *testing.T) {
	t.Parallel()

	handler := suite.New(SettingsFixture{DataDir: t.TempDir()})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/playback", strings.NewReader("mode=unknown"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("save invalid playback = %d %q", response.Code, response.Body.String())
	}
}

func (suite SettingsRegression) OwnerCanNameTheirServer(t *testing.T) {
	t.Parallel()

	mediaDir, dataDir := t.TempDir(), t.TempDir()
	handler := suite.New(SettingsFixture{MediaDir: mediaDir, DataDir: dataDir})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/server", strings.NewReader("name=Living+Room"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	home := httptest.NewRecorder()
	suite.New(SettingsFixture{MediaDir: mediaDir, DataDir: dataDir}).ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if response.Code != http.StatusSeeOther || !strings.Contains(home.Body.String(), "Living Room") {
		t.Fatalf("rename = %d, home = %q", response.Code, home.Body.String())
	}
}
