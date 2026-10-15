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

func TestAutomaticPlaybackStartsDirectWithAdaptiveFallback(t *testing.T) {
	t.Parallel()
	media, data, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\",\"profile\":\"High\",\"level\":40,\"width\":1920,\"height\":1080},{\"codec_type\":\"audio\",\"codec_name\":\"aac\",\"profile\":\"LC\"}],\"format\":{\"format_name\":\"mp4\"}}'\n")
	handler := server.New(server.Config{MediaDir: media, DataDir: data, FFprobe: ffprobe, RequireAuth: true})
	token := localPlaybackViewer(t, handler)
	homeRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	homeRequest.Header.Set("Authorization", "Bearer "+token)
	home := serveRequest(handler, homeRequest)
	match := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())
	if len(match) != 2 {
		t.Fatalf("library lacks item: %d %q", home.Code, home.Body.String())
	}
	id := match[1]

	local := httptest.NewRecorder()
	localRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil)
	localRequest.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(local, localRequest)
	if local.Code != http.StatusOK || !strings.Contains(local.Body.String(), `src="/media/`+id+`?playbackSession=`) || !strings.Contains(local.Body.String(), `data-adaptive="/hls/`) || strings.Contains(local.Body.String(), `data-hls=`) || strings.Contains(local.Body.String(), `/static/hls.min.js`) {
		t.Fatalf("local playback = %d %q", local.Code, local.Body.String())
	}

	api := apiCall(t, handler, token, http.MethodGet, "/api/v1/items/"+id+"/playback", nil)
	if api.Code != http.StatusOK || !strings.Contains(api.Body.String(), `"mode":"direct"`) || !strings.Contains(api.Body.String(), `"compatible":"/hls/`) {
		t.Fatalf("API playback = %d %q", api.Code, api.Body.String())
	}
}

func localPlaybackViewer(t *testing.T, handler http.Handler) string {
	t.Helper()
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	create := requestWithCookieRequest(t, http.MethodPost, "/settings/profiles", "name=Viewer&password=viewer-password&rating=all&libraries=all&transcode=true", owner)
	if response := serveRequest(handler, create); response.Code != http.StatusSeeOther {
		t.Fatalf("create Viewer = %d %q", response.Code, response.Body.String())
	}
	return signInTestProfile(t, handler, "/login", "name=Viewer&password=viewer-password").Value
}

func TestExplicitPlaybackModesBypassAutomaticFallbackAcrossAPIAndWeb(t *testing.T) {
	t.Parallel()
	media, data, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\",\"profile\":\"High\",\"level\":40,\"width\":1920,\"height\":1080},{\"codec_type\":\"audio\",\"codec_name\":\"aac\",\"profile\":\"LC\"}],\"format\":{\"format_name\":\"mp4\",\"duration\":\"120\"}}'\n")
	handler, id := firstWebItem(t, server.Config{MediaDir: media, DataDir: data, FFprobe: ffprobe})
	marker := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/markers/"+id, strings.NewReader("type=intro&start=10&end=20"))
	marker.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if response := serveRequest(handler, marker); response.Code != http.StatusSeeOther {
		t.Fatalf("add marker = %d %q", response.Code, response.Body.String())
	}
	setMode := func(mode string) {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/playback", strings.NewReader("mode="+mode+"&markers=intro"))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if response := serveRequest(handler, request); response.Code != http.StatusSeeOther {
			t.Fatalf("set %s mode = %d %q", mode, response.Code, response.Body.String())
		}
	}
	assertPlayback := func(mode string, expected, rejected []string) {
		setMode(mode)
		page := serveRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
		api := serveRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/"+id+"/playback", nil))
		body := page.Body.String() + api.Body.String()
		for _, value := range expected {
			if !strings.Contains(body, value) {
				t.Fatalf("%s playback lacks %q: %q", mode, value, body)
			}
		}
		for _, value := range rejected {
			if strings.Contains(body, value) {
				t.Fatalf("%s playback contains %q: %q", mode, value, body)
			}
		}
	}

	assertPlayback("direct", []string{`src="/media/` + id + `?playbackSession=`, `"mode":"direct"`, `data-marker="intro"`}, []string{`data-adaptive=`, `data-fallback=`})
	assertPlayback("compatible", []string{`data-hls="/hls/`, `"mode":"transcode"`}, []string{`data-adaptive=`})
}

func TestTrustedOpeningOffsetAcrossAPIAndWeb(t *testing.T) {
	t.Parallel()
	media, data, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\",\"profile\":\"High\",\"level\":40,\"width\":1920,\"height\":1080},{\"codec_type\":\"audio\",\"codec_name\":\"aac\",\"profile\":\"LC\"}],\"format\":{\"format_name\":\"mp4\",\"duration\":\"120\"}}'\n")
	handler, id := firstWebItem(t, server.Config{MediaDir: media, DataDir: data, FFprobe: ffprobe})
	marker := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/markers/"+id, strings.NewReader("type=intro&start=0&end=20"))
	marker.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if response := serveRequest(handler, marker); response.Code != http.StatusSeeOther {
		t.Fatalf("add marker = %d %q", response.Code, response.Body.String())
	}
	setMode := func(mode string) {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/playback", strings.NewReader("mode="+mode+"&markers=intro"))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if response := serveRequest(handler, request); response.Code != http.StatusSeeOther {
			t.Fatalf("set %s mode = %d %q", mode, response.Code, response.Body.String())
		}
	}

	setMode("direct")
	page := serveRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	api := serveRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/"+id+"/playback", nil))
	if !strings.Contains(page.Body.String(), `data-start="20"`) || !strings.Contains(page.Body.String(), `data-autoplay`) || !strings.Contains(page.Body.String(), `#t=20"`) || !strings.Contains(api.Body.String(), `"start":20`) {
		t.Fatalf("direct starts = web %d %q, API %d %q", page.Code, page.Body.String(), api.Code, api.Body.String())
	}

	setMode("compatible")
	page = serveRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	api = serveRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/"+id+"/playback", nil))
	var compatible map[string]any
	mustJSON(t, api, &compatible)
	_, hasStart := compatible["start"]
	if !strings.Contains(page.Body.String(), `data-start="0"`) || hasStart {
		t.Fatalf("server starts = web %d %q, API %d %q", page.Code, page.Body.String(), api.Code, api.Body.String())
	}

	setMode("direct")
	if progress := apiCall(t, handler, "", http.MethodPut, "/api/v1/items/"+id+"/progress", map[string]any{"seconds": 42}); progress.Code != http.StatusOK {
		t.Fatalf("save progress = %d %q", progress.Code, progress.Body.String())
	}
	page = serveRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	api = serveRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/"+id+"/playback", nil))
	if !strings.Contains(page.Body.String(), `data-start="42"`) || !strings.Contains(page.Body.String(), `data-autoplay`) || !strings.Contains(page.Body.String(), `#t=42"`) || !strings.Contains(api.Body.String(), `"start":42`) {
		t.Fatalf("resume starts = web %d %q, API %d %q", page.Code, page.Body.String(), api.Code, api.Body.String())
	}

	if progress := apiCall(t, handler, "", http.MethodPut, "/api/v1/items/"+id+"/progress", map[string]any{"seconds": 115}); progress.Code != http.StatusOK {
		t.Fatalf("save near-end progress = %d %q", progress.Code, progress.Body.String())
	}
	page = serveRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	if !strings.Contains(page.Body.String(), `data-start="115"`) || strings.Contains(page.Body.String(), `data-autoplay`) || strings.Contains(page.Body.String(), `#t=115"`) {
		t.Fatalf("near-end start = %d %q", page.Code, page.Body.String())
	}
}

func TestCompatiblePlaybackPreservesFullMovieDurationAcrossAPIAndWeb(t *testing.T) {
	t.Parallel()
	media, tools := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"hevc\"}],\"format\":{\"format_name\":\"matroska\",\"duration\":\"7200\"}}'\n")
	handler, id := firstWebItem(t, server.Config{MediaDir: media, FFprobe: ffprobe})

	api := apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback", nil)
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	if api.Code != http.StatusOK || !strings.Contains(api.Body.String(), `"duration":7200`) || page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `data-duration="7200"`) {
		t.Fatalf("API playback = %d %q, web playback = %d %q", api.Code, api.Body.String(), page.Code, page.Body.String())
	}
}

func TestMatroskaOriginalIsNotAdvertisedAsMP4AcrossAPIAndWeb(t *testing.T) {
	media, tools := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\",\"profile\":\"High\",\"level\":41},{\"codec_type\":\"audio\",\"codec_name\":\"eac3\"}],\"format\":{\"format_name\":\"matroska\"}}'\n")
	handler, id := firstWebItem(t, server.Config{MediaDir: media, FFprobe: ffprobe})

	api := apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback", nil)
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	if api.Code != http.StatusOK || !strings.Contains(api.Body.String(), `"directType":"video/x-matroska"`) || page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `data-direct-type="video/x-matroska"`) || strings.Contains(page.Body.String(), `data-direct-type="video/mp4`) {
		t.Fatalf("API playback = %d %q, web playback = %d %q", api.Code, api.Body.String(), page.Code, page.Body.String())
	}
}
