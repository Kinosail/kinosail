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

func TestPlayerOffersDirectRemotePlayback(t *testing.T) {
	t.Parallel()
	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Movie.mp4"), nil, 0o600); err != nil {
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

	for _, expected := range []string{`x-webkit-airplay="allow"`, "data-cast", "/media/" + id} {
		if !strings.Contains(player.Body.String(), expected) {
			t.Fatalf("player missing %q: %q", expected, player.Body.String())
		}
	}
	for _, expected := range []string{"player.remote", "watchAvailability", "webkitShowPlaybackTargetPicker", "webkitplaybacktargetavailabilitychanged", "webkitcurrentplaybacktargetiswirelesschanged", "connecting", "Playing on device", "Playing here", "No device selected"} {
		if !strings.Contains(script.Body.String(), expected) {
			t.Fatalf("cast script missing %q: %q", expected, script.Body.String())
		}
	}
}

func TestPlayerIntegratesWithDeviceMediaControls(t *testing.T) {
	t.Parallel()
	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Movie.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "Movie.jpg"), []byte("artwork"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	for _, expected := range []string{`data-title="Movie"`, `data-artwork="/art/` + id + `"`} {
		if !strings.Contains(player.Body.String(), expected) {
			t.Fatalf("player missing %q: %q", expected, player.Body.String())
		}
	}
	script := httptest.NewRecorder()
	handler.ServeHTTP(script, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/player.js", nil))
	for _, expected := range []string{"MediaMetadata", "setActionHandler", "setPositionState", `request("screen")`, "releaseWakeLock"} {
		if !strings.Contains(script.Body.String(), expected) {
			t.Fatalf("player script missing %q: %q", expected, script.Body.String())
		}
	}
}
