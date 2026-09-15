package servertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

type (
	TranscodeConfig  struct{ MediaDir, CacheDir, FFmpeg, FFprobe string }
	TranscodeFixture struct {
		New                       func(TranscodeConfig) http.Handler
		WriteExecutable           func(*testing.T, string, string)
		AssertSafari              func(*testing.T, http.Handler, string)
		PlayableHLS, PlayerScript string
	}
)

func (fixture TranscodeFixture) UnsupportedContainerDefaultsToDirectPlaybackWithCompatibleFallback(t *testing.T) {
	t.Helper()
	t.Parallel()

	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Brazil.mkv"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.New(TranscodeConfig{MediaDir: mediaDir})
	homeResponse := httptest.NewRecorder()
	handler.ServeHTTP(homeResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(homeResponse.Body.String())[1]
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))

	if !strings.Contains(response.Body.String(), `src="/media/`+id+`?playbackSession=`) || !strings.Contains(response.Body.String(), `data-direct-type="video/x-matroska"`) || !strings.Contains(response.Body.String(), `data-fallback="?compatible=1"`) || !strings.Contains(response.Body.String(), fixture.PlayerScript) {
		t.Fatalf("player = %q", response.Body.String())
	}
	fixture.AssertSafari(t, handler, id)
}

func (fixture TranscodeFixture) UnsupportedCodecInMP4DefaultsToDirectPlaybackWithCompatibleFallback(t *testing.T) {
	t.Helper()
	t.Parallel()

	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Brazil.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(t.TempDir(), "ffprobe")
	//nolint:gosec // G306: the fake FFprobe adapter must be executable.
	if err := os.WriteFile(ffprobe, []byte("#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"hevc\"},{\"codec_type\":\"audio\",\"codec_name\":\"eac3\"}]}'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	handler := fixture.New(TranscodeConfig{MediaDir: mediaDir, FFprobe: ffprobe})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	script := httptest.NewRecorder()
	handler.ServeHTTP(script, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/player.js", nil))

	if !strings.Contains(response.Body.String(), `src="/media/`+id+`?playbackSession=`) || !strings.Contains(response.Body.String(), `data-direct-type=""`) || !strings.Contains(response.Body.String(), `data-fallback="?compatible=1"`) || !strings.Contains(script.Body.String(), "const directSupport = directType ? player.canPlayType(directType)") {
		t.Fatalf("player = %q, script = %q", response.Body.String(), script.Body.String())
	}
}

func (fixture TranscodeFixture) ViewerCanRequestSeekableCompatiblePlaylist(t *testing.T) {
	t.Helper()
	t.Parallel()

	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Brazil.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffmpeg := filepath.Join(t.TempDir(), "ffmpeg")
	script := "#!/bin/sh\n" + fixture.PlayableHLS
	//nolint:gosec // G306: the fake ffmpeg test fixture must be executable.
	if err := os.WriteFile(ffmpeg, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(t.TempDir(), "ffprobe")
	fixture.WriteExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1080},{"codec_type":"audio","codec_name":"aac"}],"format":{"duration":"120"}}'
`)
	cacheDir := t.TempDir()
	handler := fixture.New(TranscodeConfig{MediaDir: mediaDir, CacheDir: cacheDir, FFmpeg: ffmpeg, FFprobe: ffprobe})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	seedOldPlaylist(t, cacheDir, id)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+id+"/index.m3u8", nil))

	if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), "mpegurl") ||
		!strings.Contains(response.Body.String(), "1080p/index.m3u8") {
		t.Fatalf("playlist = %d %q %q", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}
	assertFinalizedCompatiblePlaylist(t, handler, id, response)
}

func seedOldPlaylist(t *testing.T, cacheDir, id string) {
	t.Helper()
	directory := filepath.Join(cacheDir, id)
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "index.m3u8"), []byte("#EXTM3U\n#EXT-X-ENDLIST\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertFinalizedCompatiblePlaylist(t *testing.T, handler http.Handler, id string, response *httptest.ResponseRecorder) {
	t.Helper()
	variants := regexp.MustCompile(`([0-9]+p/index\.m3u8)`).FindAllStringSubmatch(response.Body.String(), -1)
	if len(variants) == 0 {
		t.Fatalf("playlist has no variants: %q", response.Body.String())
	}
	for range 50 {
		finalized := true
		for _, variant := range variants {
			response = httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+id+"/"+variant[1], nil))
			if !strings.Contains(response.Body.String(), "#EXT-X-PLAYLIST-TYPE:VOD") || !strings.Contains(response.Body.String(), "#EXT-X-ENDLIST") || strings.Contains(response.Body.String(), "#EXT-X-PLAYLIST-TYPE:EVENT") {
				finalized = false
			}
		}
		if finalized {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("playlist was not finalized: %q", response.Body.String())
}
