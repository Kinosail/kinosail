package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestVideoPlayerPresentsPlaybackStateAndTheaterControls(t *testing.T) {
	t.Parallel()
	mediaDir := t.TempDir()
	for name, content := range map[string]string{"Arrival.mp4": "video", "Arrival.jpg": "poster"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))

	for _, expected := range []string{`controls playsinline autoplay`, `poster="/art/` + id + `"`, `data-playback-api="/api/v1/items/` + id + `/playback"`, `data-playback-policy="automatic"`, `data-compatibility-mode="transcode"`, `data-player-controls`, `data-player-toggle`, `data-player-seek`, `data-player-fullscreen`, `data-player-status`, `data-player-fallback`, `data-player-settings`, `data-playback-mode-status`, `<legend>Playback policy</legend>`, `value="direct-first"`, `value="direct-only"`, `value="compatible"`, `data-playback-reason`, `data-subtitles`, `data-theater`, `aria-keyshortcuts="T"`, `class="player-stage-actions"`, `button class="quiet play-on-tv" type="button" data-tv-open aria-haspopup="dialog"`, `class="primary-action-row"`, `<summary>Playback &amp; downloads</summary>`, `<span>Playback</span><strong>`} {
		if player.Code != http.StatusOK || !strings.Contains(player.Body.String(), expected) {
			t.Fatalf("player lacks %q: %d %q", expected, player.Code, player.Body.String())
		}
	}
	body := player.Body.String()
	stage := strings.Index(body, `class="media-stage`)
	title := strings.Index(body, `class="title-block"`)
	if stage < 0 || title <= stage {
		t.Fatal("playback must precede title details in reading and focus order")
	}
	if strings.Contains(body, `data-playback-mode-status>Starting`) {
		t.Fatal("playback method must not duplicate the loading status")
	}
}

func TestVideoPlayerPresentsPictureInPictureControl(t *testing.T) {
	t.Parallel()
	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Picture-in-Picture.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	if !strings.Contains(page.Body.String(), `data-player-pip`) || !strings.Contains(page.Body.String(), `class=i-pip`) {
		t.Fatalf("player lacks Picture-in-Picture control: %q", page.Body.String())
	}
}

func TestDirectPlayerDoesNotRestartTheSourceSelectedByHTML(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{})
	script := httptest.NewRecorder()
	handler.ServeHTTP(script, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/player.js", nil))
	if !strings.Contains(script.Body.String(), `direct && !player.getAttribute("src")`) {
		t.Fatalf("player script can restart an already loading direct source: %q", script.Body.String())
	}
}

func TestPlayerNegotiatesCodecSupportBeforeStartingAdaptivePlayback(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{})
	script := httptest.NewRecorder()
	handler.ServeHTTP(script, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/player.js", nil))
	for _, expected := range []string{"navigator.mediaCapabilities", `type: "file"`, "MediaSource.isTypeSupported", "videoCodecs", "player.dataset.playbackApi", "player.dataset.mediaWidth", "player.dataset.mediaBitrate", "requestVideoFrameCallback", `artist: player.dataset.artist`, `actions.nexttrack`, `home-assistant.command`} {
		if !strings.Contains(script.Body.String(), expected) {
			t.Fatalf("player script lacks %q: %q", expected, script.Body.String())
		}
	}
}
