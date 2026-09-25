package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestDirectPlayerShowsSourceChaptersAndClientSkipMarkers(t *testing.T) { //nolint:cyclop // One rendered-player assertion covers the complete chapter and scrubber contract.
	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(t.TempDir(), "ffprobe")
	probe := `{"streams":[{"codec_type":"video","codec_name":"h264"}],"chapters":[{"start_time":"0","end_time":"90","tags":{"title":"Intro"}},{"start_time":"90","end_time":"600","tags":{"title":"First contact"}},{"start_time":"600","end_time":"660","tags":{"title":"End Credits"}}],"format":{"duration":"660"}}`
	//nolint:gosec // G306: the fake FFprobe adapter must be executable.
	if err := os.WriteFile(ffprobe, []byte("#!/bin/sh\nprintf '%s' '"+probe+"'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, FFprobe: ffprobe, CacheDir: t.TempDir()})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	script := httptest.NewRecorder()
	handler.ServeHTTP(script, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/player.js", nil))

	for _, expected := range []string{"First contact", `data-auto-skip="intro,credits"`, `data-playback-token="`, `data-chapter data-start="90" data-end="600" data-seek="90"`, `<time>1:30</time>`, `<summary><span>Chapters</span><small>3</small></summary>`, `data-marker="intro"`, `data-marker="credits"`, `controls playsinline`, `data-player-controls`, `data-player-fullscreen`, `data-seek-preview hidden`, `data-trickplay="/trickplay/` + id + `/{second}`, `/static/player.js?v=79`, `/static/app.css?v=electric-27`} {
		if !strings.Contains(player.Body.String(), expected) {
			t.Fatalf("player lacks %q: %q", expected, player.Body.String())
		}
	}
	if actions, chapters := strings.Index(player.Body.String(), `class="primary-player-actions"`), strings.Index(player.Body.String(), `class="chapters"`); actions < 0 || chapters < 0 || actions > chapters {
		t.Fatalf("primary actions must precede chapters: actions=%d chapters=%d", actions, chapters)
	}
	assertAbsent(t, player.Body.String(), `class="preview-seek"`, "Preview seek")
	for _, expected := range []string{"dataset.seek", "autoSkip.has(marker.dataset.marker)", `marker.dataset.skipped = "true"`, `button.setAttribute("aria-current", "true")`} {
		if !strings.Contains(script.Body.String(), expected) {
			t.Fatalf("script lacks %q: %q", expected, script.Body.String())
		}
	}
	if !strings.Contains(script.Body.String(), "seek.dataset.trickplay") {
		t.Fatal("seek preview is not wired to trickplay frames")
	}
}

func TestChaptersDBFallbackSharesChapterDataWithWebAndAPI(t *testing.T) {
	chaptersDB := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/chapters/123" {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write([]byte(`{"episodeId":"episode-123","episodeTitle":"S01E01 - Pilot","chapters":[{"id":"set-1","entries":[{"time":"00:00:00.000","name":"Cold open"},{"time":"00:01:00.000","name":"The case"}],"note":"approved","upvotes":1,"downvotes":0,"uploaderName":"member","createdAt":"2026-01-01T00:00:00Z"}],"count":1}`))
	}))
	t.Cleanup(chaptersDB.Close)
	mediaDir := t.TempDir()
	showDir := filepath.Join(mediaDir, "Example", "Season 01")
	if err := os.MkdirAll(showDir, 0o700); err != nil {
		t.Fatal(err)
	}
	mediaPath := filepath.Join(showDir, "Example.S01E01.mkv")
	if err := os.WriteFile(mediaPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(mediaPath, filepath.Ext(mediaPath))+".nfo", []byte(`<episodedetails><uniqueid type="tvdb">123</uniqueid></episodedetails>`), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(t.TempDir(), "ffprobe")
	probe := `{"streams":[{"codec_type":"video","codec_name":"h264"}],"format":{"format_name":"matroska","duration":"120"}}`
	//nolint:gosec // G306: the fake FFprobe adapter must be executable.
	if err := os.WriteFile(ffprobe, []byte("#!/bin/sh\nprintf '%s' '"+probe+"'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, FFprobe: ffprobe, CacheDir: t.TempDir(), Metadata: server.MetadataConfig{ChaptersURL: chaptersDB.URL + "/api/v1"}})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	playback := httptest.NewRecorder()
	handler.ServeHTTP(playback, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/"+id+"/playback", nil))
	for _, body := range []string{player.Body.String(), playback.Body.String()} {
		if !strings.Contains(body, "Cold open") || !strings.Contains(body, "The case") {
			t.Fatalf("ChaptersDB titles missing: %q", body)
		}
	}
}

func assertAbsent(t *testing.T, body string, removed ...string) {
	t.Helper()
	for _, value := range removed {
		if strings.Contains(body, value) {
			t.Fatalf("response still contains removed preview seek %q: %q", value, body)
		}
	}
}

func TestViewerCanRequestBoundedTrickplayFrame(t *testing.T) {
	t.Parallel()
	mediaDir, cacheDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	arguments, ffmpeg := filepath.Join(t.TempDir(), "arguments"), filepath.Join(t.TempDir(), "ffmpeg")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s' \"$*\" > '%s'\nfor output; do :; done\nprintf 'jpeg' > \"$output\"\n", arguments)
	//nolint:gosec // G306: the fake FFmpeg adapter must be executable.
	if err := os.WriteFile(ffmpeg, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, CacheDir: cacheDir, FFmpeg: ffmpeg})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	frame := httptest.NewRecorder()
	handler.ServeHTTP(frame, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/trickplay/"+id+"/27", nil))
	rejected := httptest.NewRecorder()
	handler.ServeHTTP(rejected, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/trickplay/"+id+"/999999", nil))
	used, err := os.ReadFile(arguments)

	if err != nil || frame.Code != http.StatusOK || frame.Header().Get("Content-Type") != "image/jpeg" || frame.Body.String() != "jpeg" || !strings.Contains(string(used), "-ss 20") || rejected.Code != http.StatusNotFound {
		t.Fatalf("frame = %d %q %q, rejected = %d, args = %q, error = %v", frame.Code, frame.Header().Get("Content-Type"), frame.Body.String(), rejected.Code, used, err)
	}
}
