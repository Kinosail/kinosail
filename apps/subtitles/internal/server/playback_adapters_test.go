package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestWebAPIAndJellyfinUseCapabilityDrivenPlaybackPlan(t *testing.T) { //nolint:cyclop,funlen // One media fixture proves the shared operation through all three adapters.
	t.Parallel()
	media, data, cache, tools := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"index":0,"codec_type":"video","codec_name":"hevc","profile":"Main 10","pix_fmt":"yuv420p10le","width":3840,"height":2160},{"index":1,"codec_type":"audio","codec_name":"eac3","channels":6,"disposition":{"default":1},"tags":{"language":"eng"}}],"format":{"format_name":"matroska,webm","bit_rate":"18000000"}}'
`)
	ffmpeg := filepath.Join(tools, "ffmpeg")
	writeExecutable(t, ffmpeg, "#!/bin/sh\n"+fakePlayableHLS())
	handler := newJellyfinServer(t, server.Config{MediaDir: media, DataDir: data, CacheDir: cache, FFprobe: ffprobe, FFmpeg: ffmpeg, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	token, _ := jellyfinLogin(t, handler, owner)
	items := jellyfinCall(t, handler, http.MethodGet, "/Items", "", token)
	var catalog jellyfinItems
	decodeJellyfin(t, items, &catalog)
	id := catalog.Items[0].ID

	assertCapabilityPlaybackWebAndAPI(t, handler, id, token, owner)

	direct := jellyfinCall(t, handler, http.MethodPost, "/Items/"+id+"/PlaybackInfo", `{"DeviceProfile":{"DirectPlayProfiles":[{"Type":"Video","Container":"mkv","VideoCodec":"hevc","AudioCodec":"eac3"}]}}`, token)
	if direct.Code != http.StatusOK || !strings.Contains(direct.Body.String(), `"SupportsDirectPlay":true`) || strings.Contains(direct.Body.String(), `"SupportsTranscoding":true`) {
		t.Fatalf("direct profile = %d %q", direct.Code, direct.Body.String())
	}
	transcode := jellyfinCall(t, handler, http.MethodPost, "/Items/"+id+"/PlaybackInfo", `{"DeviceProfile":{"DirectPlayProfiles":[{"Type":"Video","Container":"mp4","VideoCodec":"h264","AudioCodec":"aac"}],"TranscodingProfiles":[{"Type":"Video","Container":"mp4","VideoCodec":"h264","AudioCodec":"aac","Protocol":"hls"}]}}`, token)
	if transcode.Code != http.StatusOK || strings.Contains(transcode.Body.String(), `"SupportsDirectPlay":true`) || !strings.Contains(transcode.Body.String(), `"SupportsTranscoding":true`) || !strings.Contains(transcode.Body.String(), `"TranscodingUrl":"/Videos/`) {
		t.Fatalf("transcode profile = %d %q", transcode.Code, transcode.Body.String())
	}
	var delivery struct {
		MediaSources []struct {
			TranscodingURL string `json:"TranscodingUrl"`
		} `json:"MediaSources"`
	}
	decodeJellyfin(t, transcode, &delivery)
	compatible := httptest.NewRecorder()
	handler.ServeHTTP(compatible, httptest.NewRequestWithContext(t.Context(), http.MethodGet, delivery.MediaSources[0].TranscodingURL, nil))
	assertAPIBody(t, compatible, http.StatusOK, "#EXTM3U", "1080p/index.m3u8?playSessionId=")
	assertCapabilityTranscodeRenditions(t, handler, delivery.MediaSources[0].TranscodingURL, compatible.Body.String())
	assertRejectedJellyfinTranscodeChildren(t, handler, id, token, cache)
}

// Adaptive ladders depend on available encoder capacity; every advertised
// rendition must obey the client's 1080p policy and retain its capability.
func assertCapabilityTranscodeRenditions(t *testing.T, handler http.Handler, masterURL, manifest string) {
	t.Helper()
	master, err := url.Parse(masterURL)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, line := range strings.Split(manifest, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		child := capabilityRenditionURL(t, master, line)
		assertCapabilityTranscodeChild(t, handler, child)
		count++
	}
	if count == 0 {
		t.Fatal("no advertised adaptive renditions")
	}
}

func capabilityRenditionURL(t *testing.T, master *url.URL, line string) *url.URL {
	t.Helper()
	child, err := url.Parse(line)
	if err != nil || child.Scheme+child.Host+child.Fragment != "" || child.User != nil {
		t.Fatal("invalid adaptive child URL")
	}
	allowed := map[string]bool{"540p/index.m3u8": true, "720p/index.m3u8": true, "1080p/index.m3u8": true}
	if !allowed[child.Path] {
		t.Fatalf("rendition exceeds capability policy: %s", child.Path)
	}
	if len(child.Query()) != 1 || child.Query().Get("playSessionId") == "" || child.Query().Get("playSessionId") != master.Query().Get("playSessionId") {
		t.Fatal("adaptive child lost its delivery capability")
	}
	child.Path = strings.TrimSuffix(master.Path, "index.m3u8") + child.Path
	return child
}

func assertCapabilityTranscodeChild(t *testing.T, handler http.Handler, child *url.URL) {
	t.Helper()
	assertAPIBody(t, jellyfinCall(t, handler, http.MethodGet, child.Path, "", ""), http.StatusUnauthorized)
	variant := jellyfinCall(t, handler, http.MethodGet, child.String(), "", "")
	assertAPIBody(t, variant, http.StatusOK, "#EXTINF", "segment-00000.m4s?playSessionId=", "URI=\"init.mp4?playSessionId=")
	child.Path = strings.TrimSuffix(child.Path, "index.m3u8") + "segment-00000.m4s"
	segment := jellyfinCall(t, handler, http.MethodGet, child.String(), "", "")
	if segment.Code != http.StatusOK || segment.Body.String() != "segment" {
		t.Fatalf("transcode segment: %d", segment.Code)
	}
}

func TestAutomaticPlaybackStartsDirectWithAdaptiveFallback(t *testing.T) { //nolint:cyclop // One adapter matrix proves direct-first playback and its adaptive fallback.
	t.Parallel()
	media, data, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\",\"profile\":\"High\",\"level\":40,\"width\":1920,\"height\":1080},{\"codec_type\":\"audio\",\"codec_name\":\"aac\",\"profile\":\"LC\"}],\"format\":{\"format_name\":\"mp4\"}}'\n")
	handler := server.New(server.Config{MediaDir: media, DataDir: data, FFprobe: ffprobe, ProxyToken: "proxy-capability", RequireAuth: true})
	public, token := trustedProxyPublicViewer(t, handler)
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

	remoteRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil)
	remoteRequest.Header.Set("Authorization", "Bearer "+token)
	remote := httptest.NewRecorder()
	public.ServeHTTP(remote, remoteRequest)
	if remote.Code != http.StatusOK || !strings.Contains(remote.Body.String(), `src="/media/`+id+`?playbackSession=`) || !strings.Contains(remote.Body.String(), `data-adaptive="/hls/`) || strings.Contains(remote.Body.String(), `data-hls=`) || strings.Contains(remote.Body.String(), `/static/hls.min.js`) {
		t.Fatalf("remote playback = %d %q", remote.Code, remote.Body.String())
	}

	api := apiCall(t, public, token, http.MethodGet, "/api/v1/items/"+id+"/playback", nil)
	if api.Code != http.StatusOK || !strings.Contains(api.Body.String(), `"mode":"direct"`) || !strings.Contains(api.Body.String(), `"compatible":"/hls/`) || !strings.Contains(api.Body.String(), `/p/r-`) {
		t.Fatalf("remote API playback = %d %q", api.Code, api.Body.String())
	}
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

func TestTrustedOpeningOffsetAcrossAPIAndWeb(t *testing.T) { //nolint:cyclop,funlen // One contract covers the API and player adapters.
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
	api = serveRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/"+id+"/playback", nil))
	if !strings.Contains(page.Body.String(), `data-start="115"`) || !strings.Contains(page.Body.String(), `data-autoplay`) || !strings.Contains(page.Body.String(), `#t=115"`) || !strings.Contains(api.Body.String(), `"start":115`) {
		t.Fatalf("near-end starts = web %d %q, API %d %q", page.Code, page.Body.String(), api.Code, api.Body.String())
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
