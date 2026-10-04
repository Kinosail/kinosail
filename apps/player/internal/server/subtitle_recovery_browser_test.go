package server_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

// Opt-in: the real Server renders watch HTML and serves its composed player asset
// and sidecar caption bytes. A disposable HTTP wrapper injects transport stalls.
func TestSubtitleRecoveryBrowserJourney(t *testing.T) {
	if os.Getenv("KINOSAIL_CAPTION_BROWSER") != "1" {
		t.Skip("KINOSAIL_CAPTION_BROWSER is not set")
	}
	clip, err := os.ReadFile(os.Getenv("KINOSAIL_CAPTION_MEDIA_FIXTURE"))
	if err != nil {
		t.Fatal("provide KINOSAIL_CAPTION_MEDIA_FIXTURE with a disposable playable MP4:", err)
	}
	t.Logf("playable fixture SHA-256: %x", sha256.Sum256(clip))
	media := t.TempDir()
	for name, data := range map[string][]byte{
		"Caption Recovery.mp4":    clip,
		"Caption Recovery.en.vtt": []byte("WEBVTT\n\n00:00:01.000 --> 00:01:00.000\nRecovered captions\n"),
		"Caption Recovery.es.vtt": []byte("WEBVTT\n\n00:00:01.000 --> 00:01:00.000\nSpanish captions\n"),
	} {
		if err := os.WriteFile(filepath.Join(media, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: media, DataDir: t.TempDir(), FFmpeg: "/unavailable-caption-fixture", FFprobe: "/unavailable-caption-fixture"})
	settings := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/playback", strings.NewReader("mode=direct&subtitles=on"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(settings, request)
	if settings.Code != http.StatusSeeOther {
		t.Fatalf("disposable caption preferences = %d", settings.Code)
	}
	faults := &captionFaultPeer{handler: handler}
	web := httptest.NewServer(faults)
	t.Cleanup(web.Close)
	project := os.Getenv("KINOSAIL_BROWSER_PROJECT")
	if project == "" {
		project = "chromium"
	}
	command := exec.CommandContext(t.Context(), "pnpm", "exec", "playwright", "test", "player-subtitles-recovery.spec.ts", "--workers=1", "--project="+project) //nolint:gosec // Fixed test command against a disposable local Server.
	command.Dir = "../../e2e"
	command.Env = append(os.Environ(), "KINOSAIL_CAPTION_BROWSER_URL="+web.URL, "KINOSAIL_E2E_URL="+web.URL)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
}

type captionFaultPeer struct {
	handler http.Handler
	mutex   sync.Mutex
	mode    string
	calls   int
	closed  int
}

func (peer *captionFaultPeer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/__caption-fixture" {
		peer.mutex.Lock()
		defer peer.mutex.Unlock()
		if request.Method == http.MethodPost {
			mode := request.URL.Query().Get("mode")
			if mode != "headers" && mode != "body" && mode != "normal" {
				http.Error(writer, "Invalid fixture mode", http.StatusBadRequest)
				return
			}
			peer.mode, peer.calls, peer.closed = mode, 0, 0
		}
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(map[string]int{"calls": peer.calls, "closed": peer.closed}); err != nil {
			return
		}
		return
	}
	if !strings.HasPrefix(request.URL.Path, "/subtitle/") {
		peer.handler.ServeHTTP(writer, request)
		return
	}
	peer.mutex.Lock()
	peer.calls++
	mode := peer.mode
	peer.mode = "normal"
	peer.mutex.Unlock()
	if mode != "headers" && mode != "body" {
		peer.handler.ServeHTTP(writer, request)
		return
	}
	if mode == "body" {
		writer.Header().Set("Content-Type", "text/vtt")
		writer.Header().Set("X-Request-ID", "caption-fixture-1")
		if _, err := fmt.Fprint(writer, "WEBVTT\n\n"); err != nil {
			return
		}
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}
	}
	<-request.Context().Done()
	peer.mutex.Lock()
	peer.closed++
	peer.mutex.Unlock()
}
