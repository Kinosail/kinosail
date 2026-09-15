package server

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestHLSDiagnosticErrorBoundsAndRedactsPrivatePaths(t *testing.T) {
	media := "/private/library/Private Title.mkv"
	cache := "/private/cache/session"
	detail := strings.Repeat("x", hlsDiagnosticLimit) + " invalid filter for " + media + " and " + cache + "/index.m3u8"
	err := newHLSDiagnosticError(errors.New("exit status 1"), detail, media, cache)
	if err.Error() != "compatible playback failed" || strings.Contains(err.Detail(), "Private Title") || strings.Contains(err.Detail(), "/private/") || len(err.Detail()) > hlsDiagnosticLimit {
		t.Fatalf("unsafe diagnostic error: public=%q detail=%q", err.Error(), err.Detail())
	}
}

func TestHLSDiagnosticPreservesCauseAndKeepsOnlyRecentOutput(t *testing.T) {
	cause := errors.New("transcoder failed")
	failure := newHLSDiagnosticError(cause, "detail")
	servertest.HLSDiagnosticPreservesCauseAndKeepsOnlyRecentOutput(t, cause, failure, new(hlsDiagnosticBuffer), hlsDiagnosticLimit)
}

func TestHLSFailureIsCorrelatedAndPrivate(t *testing.T) {
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	mediaPath := filepath.Join(media, "Private Title.mkv")
	if err := os.WriteFile(mediaPath, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffmpeg := filepath.Join(tools, "ffmpeg")
	ffprobe := filepath.Join(tools, "ffprobe")
	if err := os.WriteFile(ffprobe, []byte("#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\",\"width\":640,\"height\":360},{\"codec_type\":\"audio\",\"codec_name\":\"aac\"}],\"format\":{\"format_name\":\"mp4\",\"duration\":\"60\"}}'\n"), 0o700); err != nil { //nolint:gosec // The fake FFprobe adapter must be executable.
		t.Fatal(err)
	}
	if err := os.WriteFile(ffmpeg, []byte("#!/bin/sh\nprintf 'invalid filter for %s\\n' '"+mediaPath+"' >&2\nexit 1\n"), 0o700); err != nil { //nolint:gosec // The fake FFmpeg adapter must be executable.
		t.Fatal(err)
	}
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	handler := New(Config{MediaDir: media, CacheDir: cache, FFprobe: ffprobe, FFmpeg: ffmpeg})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=movies", nil))
	match := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())
	if home.Code != http.StatusOK || len(match) != 2 {
		t.Fatal("Movies view did not expose the diagnostic media fixture")
	}
	id := match[1]
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+id+"/index.m3u8", nil)
	request.Header.Set("X-Request-ID", "diagnostic-request")
	request.Header.Set("X-Playback-Session", "playback-diagnostic")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("response=%d %q", response.Code, response.Body.String())
	}
	for _, field := range []string{`"msg":"HLS transcode failed"`, `"request_id":"diagnostic-request"`, `"playback_session":"playback-diagnostic"`, `"route":"GET /hls/{id}/{file...}"`} {
		if !strings.Contains(output.String(), field) {
			t.Fatalf("missing %s in %s", field, output.String())
		}
	}
	assertHLSFailurePrivacy(t, response, &output, []string{mediaPath, "Private Title", id})
}

func assertHLSFailurePrivacy(t *testing.T, response *httptest.ResponseRecorder, output *bytes.Buffer, privateValues []string) {
	t.Helper()
	for _, private := range privateValues {
		if strings.Contains(response.Body.String(), private) || strings.Contains(output.String(), private) {
			t.Fatalf("HLS failure exposed %q: response=%q log=%s", private, response.Body.String(), output.String())
		}
	}
}
