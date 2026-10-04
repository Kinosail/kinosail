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
	"sync"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

// Opt-in: native browse, Show, watch, and Back journeys use an unmodified real
// Server with fictional files. The outer peer observes requests only.
func TestBrowseReturnBrowserJourney(t *testing.T) {
	if os.Getenv("KINOSAIL_BROWSE_RETURN_BROWSER") != "1" {
		t.Skip("KINOSAIL_BROWSE_RETURN_BROWSER is not set")
	}
	mode := os.Getenv("KINOSAIL_BROWSE_RETURN_CASES")
	if mode == "" {
		mode = "primary"
	}
	switch mode {
	case "primary", "cold", "bfcache", "htmx", "shows", "search", "all":
	default:
		t.Fatalf("invalid browse-return case selection: %q", mode)
	}
	clip, err := os.ReadFile(os.Getenv("KINOSAIL_BROWSE_RETURN_MEDIA")) //nolint:gosec // Explicit disposable fixture selected by the runner; never a production media path.
	if err != nil {
		t.Fatal("provide KINOSAIL_BROWSE_RETURN_MEDIA with the authorized fictional MP4:", err)
	}
	if len(clip) == 0 || len(clip) > 8*1024*1024 {
		t.Fatal("fictional MP4 must contain 1..8 MiB")
	}
	t.Logf("browse-return fixture: mode=%s clipBytes=%d clipSHA256=%x", mode, len(clip), sha256.Sum256(clip))
	media := t.TempDir()
	write := func(name string, playable bool) {
		t.Helper()
		path := filepath.Join(media, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		var data []byte
		if playable {
			data = clip
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for number := 1; number <= 32; number++ {
		write(fmt.Sprintf("Return Movie %02d.mp4", number), number == 25)
	}
	for number := 1; number <= 12; number++ {
		show := fmt.Sprintf("Return Show %02d", number)
		write(filepath.Join(show, "Season 01", show+" S01E01 Pilot.mp4"), number == 11)
	}
	write("Anchor Movie.mp4", false)
	write("Zeta Movie.mp4", false)
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), FFmpeg: "/unavailable-browse-return-fixture", FFprobe: "/unavailable-browse-return-fixture"})
	peer := &browseReturnPeer{handler: handler}
	web := httptest.NewServer(peer)
	t.Cleanup(web.Close)
	project := os.Getenv("KINOSAIL_BROWSER_PROJECT")
	if project == "" {
		project = "chromium"
	}
	command := exec.CommandContext(t.Context(), "pnpm", "exec", "playwright", "test", "browse-return.spec.ts", "--workers=1", "--retries=0", "--project="+project) //nolint:gosec // Fixed command and explicitly disposable local Server.
	command.Dir = "../../e2e"
	command.Env = append(os.Environ(), "KINOSAIL_BROWSE_RETURN_URL="+web.URL, "KINOSAIL_BROWSE_RETURN_CASES="+mode, "KINOSAIL_E2E_URL="+web.URL)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
}

type browseReturnRequest struct {
	Values       map[string]string `json:"values"`
	Continuation bool              `json:"continuation"`
	History      bool              `json:"history"`
	HTMX         bool              `json:"htmx"`
}

type browseReturnPeer struct {
	handler  http.Handler
	mutex    sync.Mutex
	requests []browseReturnRequest
}

func (peer *browseReturnPeer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/__browse-return" && request.Method == http.MethodGet {
		peer.mutex.Lock()
		defer peer.mutex.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(peer.requests)
		return
	}
	if request.URL.Path == "/" && request.Method == http.MethodGet {
		values := map[string]string{}
		for _, key := range []string{"view", "q", "sort", "offset", "limit", "lang", "letter"} {
			if value := request.URL.Query().Get(key); value != "" && len(value) <= 512 {
				values[key] = value
			}
		}
		peer.mutex.Lock()
		if len(peer.requests) < 256 {
			peer.requests = append(peer.requests, browseReturnRequest{Values: values, Continuation: request.Header.Get("X-Kinosail-Library-Page") == "1", History: request.Header.Get("HX-History-Restore-Request") == "true", HTMX: request.Header.Get("HX-Request") == "true"})
		}
		peer.mutex.Unlock()
	}
	peer.handler.ServeHTTP(writer, request)
}
