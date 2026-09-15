package servertest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func (fixture TranscodeFixture) InterruptedCompatibleCacheIsRegeneratedAfterServerRestart(t *testing.T) { //nolint:funlen // Cancellation, restart, and regeneration must share the same server cache.
	t.Helper()
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffmpeg := filepath.Join(tools, "ffmpeg")
	ffprobe := filepath.Join(tools, "ffprobe")
	fixture.WriteExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\",\"width\":640,\"height\":360},{\"codec_type\":\"audio\",\"codec_name\":\"aac\"}],\"format\":{\"duration\":\"8\"}}'\n")
	fixture.WriteExecutable(t, ffmpeg, "#!/bin/sh\n"+fixture.PlayableHLS)
	handler, id := FirstWebItem(t, fixture.New(TranscodeConfig{MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe}))
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id+"?compatible=1", nil))
	source := regexp.MustCompile(`data-hls="([^"]+)"`).FindStringSubmatch(page.Body.String())[1]
	master := httptest.NewRecorder()
	handler.ServeHTTP(master, httptest.NewRequestWithContext(t.Context(), http.MethodGet, source, nil))
	variant := regexp.MustCompile(`(?m)^([0-9]+p/index\.m3u8)$`).FindStringSubmatch(master.Body.String())[1]
	variantSource, _ := url.Parse(source)
	variantSource.Path = strings.TrimSuffix(variantSource.Path, "index.m3u8") + variant
	variantURL := variantSource.String()

	playlist := findTranscodeVariant(t, cache, variant)
	writeTranscodeCache(t, playlist, []byte("#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:4,\nsegment-00000.m4s\n"))
	calls := filepath.Join(tools, "calls")
	fixture.WriteExecutable(t, ffmpeg, "#!/bin/sh\nprintf 'run\\n' >> '"+calls+"'\n"+fixture.PlayableHLS)
	restarted := fixture.New(TranscodeConfig{MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe})
	assertRegeneratedVariant(t, restarted, variantURL, calls)

	writeTranscodeCache(t, playlist, []byte("#EXTM3U\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:4,\nsegment-00000.m4s\nunexpected.bin\n#EXT-X-ENDLIST\n"))
	removeTranscodeCache(t, calls)
	fixture.assertRegeneratedMaster(t, TranscodeConfig{MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe}, source, calls, "malformed cache master = %d %q, FFmpeg calls = %q")

	writeTranscodeCache(t, playlist, []byte("#EXTM3U\n#INVALID:#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:4,\nsegment-00000.m4s\n#INVALID:#EXT-X-ENDLIST\n"))
	removeTranscodeCache(t, calls)
	fixture.assertRegeneratedMaster(t, TranscodeConfig{MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe}, source, calls, "lookalike tags master = %d %q, FFmpeg calls = %q")

	removeTranscodeCache(t, filepath.Join(filepath.Dir(playlist), "segment-00000.m4s"))
	removeTranscodeCache(t, calls)
	fixture.assertRegeneratedMaster(t, TranscodeConfig{MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe}, source, calls, "missing segment master = %d %q, FFmpeg calls = %q")

	writeTranscodeCache(t, filepath.Join(filepath.Dir(playlist), "init.mp4"), nil)
	removeTranscodeCache(t, calls)
	fixture.assertRegeneratedMaster(t, TranscodeConfig{MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe}, source, calls, "empty init segment master = %d %q, FFmpeg calls = %q")

	segment := filepath.Join(filepath.Dir(playlist), "segment-00000.m4s")
	removeTranscodeCache(t, segment)
	if err := os.Symlink(filepath.Join(media, "Film.mkv"), segment); err != nil {
		t.Fatal(err)
	}
	removeTranscodeCache(t, calls)
	fixture.assertRegeneratedMaster(t, TranscodeConfig{MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe}, source, calls, "linked segment master = %d %q, FFmpeg calls = %q")
}

func (fixture TranscodeFixture) assertRegeneratedMaster(t *testing.T, config TranscodeConfig, source, calls, message string) {
	t.Helper()
	restarted := fixture.New(config)
	master := httptest.NewRecorder()
	restarted.ServeHTTP(master, httptest.NewRequestWithContext(t.Context(), http.MethodGet, source, nil))
	used, _ := os.ReadFile(calls)
	if master.Code != http.StatusOK || len(used) == 0 {
		t.Fatalf(message, master.Code, master.Body.String(), used)
	}
}

func writeTranscodeCache(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

func findTranscodeVariant(t *testing.T, cache, variant string) string {
	t.Helper()
	var playlist string
	if err := filepath.WalkDir(cache, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(path, filepath.FromSlash(variant)) {
			playlist = path
		}
		return err
	}); err != nil || playlist == "" {
		t.Fatalf("generated variant = %q, walk error = %v", playlist, err)
	}
	return playlist
}

func assertRegeneratedVariant(t *testing.T, restarted http.Handler, variantURL, calls string) {
	t.Helper()
	response := httptest.NewRecorder()
	restarted.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, variantURL, nil))
	var used []byte
	for range 50 {
		used, _ = os.ReadFile(calls)
		response = httptest.NewRecorder()
		restarted.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, variantURL, nil))
		if len(used) > 0 && strings.Contains(response.Body.String(), "#EXT-X-ENDLIST") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if response.Code != http.StatusOK || len(used) == 0 || !strings.Contains(response.Body.String(), "#EXT-X-PLAYLIST-TYPE:VOD") || !strings.Contains(response.Body.String(), "#EXT-X-ENDLIST") || strings.Contains(response.Body.String(), "#EXT-X-PLAYLIST-TYPE:EVENT") {
		t.Fatalf("variant = %d %q, FFmpeg calls = %q", response.Code, response.Body.String(), used)
	}
}

func removeTranscodeCache(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}
