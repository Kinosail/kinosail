package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestWebAPIAndJellyfinUseCapabilityDrivenPlaybackPlan(t *testing.T) { //nolint:cyclop,funlen // One media fixture proves the shared operation through all three adapters.
	t.Parallel()
	media, data, cache, tools := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"index":0,"codec_type":"video","codec_name":"hevc","profile":"Main 10","pix_fmt":"yuv420p10le","width":3840,"height":2160,"color_transfer":"smpte2084"},{"index":1,"codec_type":"audio","codec_name":"eac3","channels":6,"disposition":{"default":1},"tags":{"language":"eng"}}],"format":{"format_name":"matroska,webm","bit_rate":"18000000","duration":"9"}}'
`)
	arguments, ffmpeg := filepath.Join(tools, "arguments"), filepath.Join(tools, "ffmpeg")
	writeExecutable(t, ffmpeg, "#!/bin/sh\nprintf '%s\\n' \"$*\" > '"+arguments+"'\n"+fakePlayableHLS())
	config := server.Config{MediaDir: media, DataDir: data, CacheDir: cache, FFprobe: ffprobe, FFmpeg: ffmpeg, RequireAuth: true}
	handler := newJellyfinServer(t, config)
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	token, _ := jellyfinLogin(t, handler, owner)
	items := jellyfinCall(t, handler, http.MethodGet, "/Items", "", token)
	var catalog jellyfinItems
	decodeJellyfin(t, items, &catalog)
	id := catalog.Items[0].ID

	assertCapabilityPlaybackWebAndAPI(t, handler, id, token, owner)

	direct := jellyfinCall(t, handler, http.MethodPost, "/Items/"+id+"/PlaybackInfo", `{"DeviceProfile":{"DirectPlayProfiles":[{"Type":"Video","Container":"mkv","VideoCodec":"hevc","AudioCodec":"eac3"}],"CodecProfiles":[{"Type":"Video","Codec":"hevc","Conditions":[{"Condition":"Equals","Property":"VideoRangeType","Value":"HDR10"}]}]}}`, token)
	if direct.Code != http.StatusOK || !strings.Contains(direct.Body.String(), `"SupportsDirectPlay":true`) || strings.Contains(direct.Body.String(), `"SupportsTranscoding":true`) {
		t.Fatalf("direct profile = %d %q", direct.Code, direct.Body.String())
	}
	transcode := jellyfinCall(t, handler, http.MethodPost, "/Items/"+id+"/PlaybackInfo", `{"DeviceProfile":{"DirectPlayProfiles":[{"Type":"Video","Container":"mp4","VideoCodec":"h264","AudioCodec":"aac"}],"TranscodingProfiles":[{"Type":"Video","Container":"mp4","VideoCodec":"h264","AudioCodec":"aac","Protocol":"hls"}]}}`, token)
	if transcode.Code != http.StatusOK || strings.Contains(transcode.Body.String(), `"SupportsDirectPlay":true`) || !strings.Contains(transcode.Body.String(), `"SupportsTranscoding":true`) || !strings.Contains(transcode.Body.String(), `"TranscodingUrl":"/Videos/`) || !strings.Contains(transcode.Body.String(), `"TranscodingSubProtocol":"hls"`) || strings.Contains(transcode.Body.String(), `"TranscodingProtocol"`) {
		t.Fatalf("transcode profile = %d %q", transcode.Code, transcode.Body.String())
	}
	var delivery struct {
		PlaySessionID string `json:"PlaySessionId"`
		MediaSources  []struct {
			TranscodingURL string `json:"TranscodingUrl"`
		} `json:"MediaSources"`
	}
	decodeJellyfin(t, transcode, &delivery)
	compatible := httptest.NewRecorder()
	handler.ServeHTTP(compatible, httptest.NewRequestWithContext(t.Context(), http.MethodGet, delivery.MediaSources[0].TranscodingURL, nil))
	used := readTestFile(t, arguments)
	if compatible.Code != http.StatusOK || !strings.Contains(compatible.Body.String(), "#EXTM3U") || !strings.Contains(compatible.Body.String(), "1080p/index.m3u8?") || strings.Contains(compatible.Body.String(), "360p/index.m3u8?") || strings.Count(used, "-threads:v ") != 1 || !strings.Contains(used, "-threads:v "+strconv.Itoa(runtime.GOMAXPROCS(0))) || !strings.Contains(used, "-readrate_initial_burst 12 -readrate 1") || !strings.Contains(used, "-hls_time 2") || !strings.Contains(compatible.Body.String(), "api_key=") || !strings.Contains(compatible.Body.String(), "playSessionId=") {
		t.Fatalf("transcode delivery = %d %q", compatible.Code, compatible.Body.String())
	}
	assertRestartedJellyfinHLSChain(t, config, delivery.MediaSources[0].TranscodingURL)
	assertJellyfinSourceHLSChain(t, handler, id, delivery.PlaySessionID)
	assertJellyfinSessionHLSChain(t, handler, id, delivery.PlaySessionID)
	variantURL := strings.Replace(delivery.MediaSources[0].TranscodingURL, "/index.m3u8", "/1080p/index.m3u8", 1)
	noCapabilityVariant := jellyfinCall(t, handler, http.MethodGet, strings.Split(variantURL, "?")[0], "", "")
	if noCapabilityVariant.Code != http.StatusUnauthorized {
		t.Fatalf("transcode variant without capability = %d %q", noCapabilityVariant.Code, noCapabilityVariant.Body.String())
	}
	variant := jellyfinCall(t, handler, http.MethodGet, variantURL, "", "")
	if variant.Code != http.StatusOK || !strings.Contains(variant.Body.String(), "#EXT-X-PLAYLIST-TYPE:VOD") || !strings.Contains(variant.Body.String(), "#EXT-X-ENDLIST") || !strings.Contains(variant.Body.String(), "segment-00001.m4s?") || !strings.Contains(variant.Body.String(), `URI="init.mp4?`) || !strings.Contains(variant.Body.String(), "api_key=") || !strings.Contains(variant.Body.String(), "playSessionId=") {
		t.Fatalf("transcode variant = %d %q", variant.Code, variant.Body.String())
	}
	segment := jellyfinCall(t, handler, http.MethodGet, strings.Replace(variantURL, "/index.m3u8", "/segment-00000.m4s", 1), "", "")
	if segment.Code != http.StatusOK || segment.Header().Get("Content-Type") != "video/mp4" || segment.Body.String() != "segment" {
		t.Fatalf("transcode segment = %d %q", segment.Code, segment.Body.String())
	}
	assertRejectedJellyfinTranscodeChildren(t, handler, id, token, cache)
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
	match := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())
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
	if !strings.Contains(page.Body.String(), `data-start="0"`) || hasStart || compatible["directAllowed"] != false {
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
	if api.Code != http.StatusOK || !strings.Contains(api.Body.String(), `"directAllowed":true`) || !strings.Contains(api.Body.String(), `"media":{`) || !strings.Contains(api.Body.String(), `"Codec":"h264"`) || !strings.Contains(api.Body.String(), `"directType":"video/x-matroska"`) || page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `data-direct-type="video/x-matroska"`) || strings.Contains(page.Body.String(), `data-direct-type="video/mp4`) {
		t.Fatalf("API playback = %d %q, web playback = %d %q", api.Code, api.Body.String(), page.Code, page.Body.String())
	}
}
