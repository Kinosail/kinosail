package server_test

import (
	"encoding/json"
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

// Native WebKit media bypasses Playwright routing. Hold real HTTP bytes outside
// the real Server instead; media events, commands and persistence remain native.
func TestQueueLoadingBrowserJourney(t *testing.T) {
	if os.Getenv("KINOSAIL_QUEUE_LOADING_BROWSER") != "1" {
		t.Skip("KINOSAIL_QUEUE_LOADING_BROWSER is not set")
	}
	media := t.TempDir()
	for _, fixture := range []string{"audio-queue", "queue-loading-intent"} {
		command := exec.CommandContext(t.Context(), "python3", "../../../../scripts/testing/prepare-"+fixture+"-fixture.py", filepath.Join(media, fixture)) //nolint:gosec // Fixed generators in a disposable directory.
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("audio fixture: %v: %s", err, output)
		} else {
			t.Logf("audio fixture: %s", output)
		}
	}
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir()})
	peer := &queueLoadingPeer{handler: handler, allowed: map[string]bool{}}
	read := func(path string, value interface{}) {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), value) != nil {
			t.Fatal("real audio catalog unavailable")
		}
	}
	var catalog struct{ Albums []struct{ ID string } }
	read("/api/v1/albums", &catalog)
	for _, album := range catalog.Albums {
		var detail struct{ Tracks []struct{ Stream string } }
		read("/api/v1/albums/"+album.ID, &detail)
		for _, track := range detail.Tracks {
			peer.allowed[track.Stream] = true
		}
	}
	if len(peer.allowed) != 4 {
		t.Fatal("expected four real fictional tracks")
	}
	web := httptest.NewServer(peer)
	t.Cleanup(web.Close)
	t.Cleanup(peer.release)
	project := os.Getenv("KINOSAIL_BROWSER_PROJECT")
	if project == "" {
		project = "chromium"
	}
	command := exec.CommandContext(t.Context(), "pnpm", "exec", "playwright", "test", "test-instance-queue-loading-intent.spec.ts", "--workers=1", "--project="+project, "--retries=0", "--forbid-only") //nolint:gosec // Fixed opt-in journey against a disposable Server.
	command.Dir = "../../e2e"
	command.Env = append(os.Environ(), "KINOSAIL_TEST_INSTANCE=1", "KINOSAIL_QUEUE_LOADING_URL="+web.URL, "KINOSAIL_E2E_URL="+web.URL)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
}

type queueLoadingPeer struct {
	handler http.Handler
	allowed map[string]bool
	mutex   sync.Mutex
	path    string
	gate    chan struct{}
	held    int
}

func (peer *queueLoadingPeer) release() {
	peer.mutex.Lock()
	defer peer.mutex.Unlock()
	if peer.gate != nil {
		close(peer.gate)
	}
	peer.path, peer.gate = "", nil
}

func (peer *queueLoadingPeer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	const control = "/__queue-media"
	if strings.HasPrefix(request.URL.Path, control) {
		peer.mutex.Lock()
		defer peer.mutex.Unlock()
		path := strings.TrimPrefix(request.URL.Path, control)
		if len(request.URL.Path) > 256 || request.URL.RawPath != "" || request.URL.RawQuery != "" || request.ContentLength != 0 || (path != "" && !peer.allowed[path]) {
			writer.WriteHeader(http.StatusBadRequest)
		} else if request.Method == http.MethodGet && path == "" {
			writer.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(writer).Encode(map[string]interface{}{"path": peer.path, "held": peer.held})
		} else if request.Method == http.MethodPut && path != "" && peer.gate == nil {
			peer.path, peer.gate, peer.held = path, make(chan struct{}), 0
			writer.WriteHeader(http.StatusNoContent)
		} else if request.Method == http.MethodDelete && path == peer.path && peer.gate != nil {
			close(peer.gate)
			peer.path, peer.gate = "", nil
			writer.WriteHeader(http.StatusNoContent)
		} else {
			writer.WriteHeader(http.StatusConflict)
		}
		return
	}
	peer.mutex.Lock()
	gate := peer.gate
	if request.URL.Path == peer.path && gate != nil {
		peer.held++
	} else {
		gate = nil
	}
	peer.mutex.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-request.Context().Done():
			return
		}
	}
	peer.handler.ServeHTTP(writer, request)
}
