package server_test

import (
	"io"
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

func TestViewerCanOpenHome(t *testing.T) {
	t.Parallel()

	testServer := httptest.NewServer(server.New(server.Config{}))
	t.Cleanup(testServer.Close)

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	})
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "Kinosail") {
		t.Fatalf("home = %d %q", response.StatusCode, body)
	}
}

func TestViewerCanOpenPlayer(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	homeResponse := httptest.NewRecorder()
	handler.ServeHTTP(homeResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	match := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(homeResponse.Body.String())
	if len(match) != 2 {
		t.Fatalf("home has no watch link: %q", homeResponse.Body.String())
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+match[1], nil))

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `<video`) || strings.Contains(response.Body.String(), `aria-label="Main navigation"`) {
		t.Fatalf("player = %d %q", response.Code, response.Body.String())
	}
}

func TestUnknownPlayerStartsDirectWithCompatibleFallback(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "UnknownCodec.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	script := httptest.NewRecorder()
	handler.ServeHTTP(script, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/player.js", nil))

	if !strings.Contains(player.Body.String(), `src="/media/`) || !strings.Contains(player.Body.String(), `data-adaptive="/hls/`) || !strings.Contains(player.Body.String(), `data-direct="/media/`) || !strings.Contains(script.Body.String(), "canPlayType") {
		t.Fatalf("player = %q, script = %q", player.Body.String(), script.Body.String())
	}
}

func TestAdaptivePlayerUsesInBandStartupMeasurementAndSeamlessQualityControls(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{})
	script := httptest.NewRecorder()
	handler.ServeHTTP(script, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/player.js", nil))

	for _, expected := range []string{
		"startLevel: -1", "testBandwidth: true", "startFragPrefetch: true",
		"capLevelToPlayerSize: true", "capLevelOnFPSDrop: true", "abrMaxWithRealBitrate: true",
		"Hls.Events.MANIFEST_PARSED", "Hls.Events.LEVEL_SWITCHED", "hls.loadLevel = -1", "hls.nextLevel =",
		"networkRecoveries++ < 2", "Auto · recovered", `preference === "auto" ? "Auto" : preference`,
	} {
		if !strings.Contains(script.Body.String(), expected) {
			t.Fatalf("player script lacks %q: %q", expected, script.Body.String())
		}
	}
}

func TestViewerCanStreamMediaRange(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Range.mp4"), []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	homeResponse := httptest.NewRecorder()
	handler.ServeHTTP(homeResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(homeResponse.Body.String())[1]
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/media/"+id, nil)
	request.Header.Set("Range", "bytes=2-5")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusPartialContent || response.Body.String() != "2345" {
		t.Fatalf("range = %d %q", response.Code, response.Body.String())
	}
}

func TestViewerCanUseSidecarSubtitles(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	for name, content := range map[string]string{
		"Moon.mp4": "video",
		"Moon.srt": "1\r\n00:00:00,000 --> 00:00:01,000\r\nHello\r\n",
	} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	homeResponse := httptest.NewRecorder()
	handler.ServeHTTP(homeResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(homeResponse.Body.String())[1]
	playerResponse := httptest.NewRecorder()
	handler.ServeHTTP(playerResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	subtitleResponse := httptest.NewRecorder()
	handler.ServeHTTP(subtitleResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/subtitle/"+id, nil))

	if !strings.Contains(playerResponse.Body.String(), `<track`) || subtitleResponse.Header().Get("Content-Type") != "text/vtt; charset=utf-8" ||
		!strings.Contains(subtitleResponse.Body.String(), "WEBVTT\n\n1\n00:00:00.000 --> 00:00:01.000\nHello") {
		t.Fatalf("player = %q, subtitle = %q", playerResponse.Body.String(), subtitleResponse.Body.String())
	}
}

func TestViewerCanSeeSidecarArtwork(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	for name, content := range map[string]string{"Alien.mp4": "video", "Alien.jpg": "poster"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	homeResponse := httptest.NewRecorder()
	handler.ServeHTTP(homeResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	match := regexp.MustCompile(`/art/([a-f0-9]+)`).FindStringSubmatch(homeResponse.Body.String())
	if len(match) != 2 {
		t.Fatalf("home has no artwork: %q", homeResponse.Body.String())
	}
	artResponse := httptest.NewRecorder()
	handler.ServeHTTP(artResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/art/"+match[1], nil))
	if artResponse.Body.String() != "poster" {
		t.Fatalf("art = %q", artResponse.Body.String())
	}
}

func TestViewerCanResumePlayback(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Primer.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir()})
	homeResponse := httptest.NewRecorder()
	handler.ServeHTTP(homeResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(homeResponse.Body.String())[1]
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/progress/"+id, strings.NewReader(url.Values{"seconds": {"42"}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	playerResponse := httptest.NewRecorder()
	handler.ServeHTTP(playerResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))

	if !strings.Contains(playerResponse.Body.String(), `data-start="42"`) {
		t.Fatalf("player = %q", playerResponse.Body.String())
	}
}

func TestViewerCanFindContinuedMedia(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Heat.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir()})
	homeResponse := httptest.NewRecorder()
	handler.ServeHTTP(homeResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(homeResponse.Body.String())[1]
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/progress/"+id, strings.NewReader("seconds=60"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	homeResponse = httptest.NewRecorder()
	handler.ServeHTTP(homeResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if !strings.Contains(homeResponse.Body.String(), "Continue watching") || !strings.Contains(homeResponse.Body.String(), "Resume at 1m") {
		t.Fatalf("home = %q", homeResponse.Body.String())
	}
}

func TestHealthReportsReady(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	server.New(server.Config{}).ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "{\"status\":\"ok\"}\n" {
		t.Fatalf("health = %d %q", response.Code, response.Body.String())
	}
}

func TestResponsesCarryBrowserSecurityPolicy(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	server.New(server.Config{}).ServeHTTP(response, request)

	if !strings.Contains(response.Header().Get("Content-Security-Policy"), "default-src 'self'") || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("security headers = %v", response.Header())
	}
}

func TestViewerCanBrowseScannedLibrary(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "The.Matrix.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	server.New(server.Config{MediaDir: mediaDir}).ServeHTTP(response, request)

	if !strings.Contains(response.Body.String(), "The Matrix") {
		t.Fatalf("home = %q", response.Body.String())
	}
}

func TestViewerCanSearchLibrary(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	for _, name := range []string{"Arrival.mp4", "The.Matrix.mp4"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?q=arrival", nil)
	response := httptest.NewRecorder()
	server.New(server.Config{MediaDir: mediaDir}).ServeHTTP(response, request)

	body := response.Body.String()
	if !strings.Contains(body, "Arrival") || strings.Contains(body, "The Matrix") {
		t.Fatalf("search = %q", body)
	}
}

func TestOwnerCanManuallySyncLibrary(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	handler := server.New(server.Config{MediaDir: mediaDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if err := os.WriteFile(filepath.Join(mediaDir, "Dune.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/scan", nil))
	refreshed := httptest.NewRecorder()
	handler.ServeHTTP(refreshed, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	if !strings.Contains(home.Body.String(), ">Sync</button>") || response.Code != http.StatusSeeOther || !strings.Contains(refreshed.Body.String(), "Dune") {
		t.Fatalf("home = %q, sync = %d, refreshed = %q", home.Body.String(), response.Code, refreshed.Body.String())
	}
}
