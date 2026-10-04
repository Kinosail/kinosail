package server_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

// Opt-in: public original-quality preparation and Go-rendered Downloads use real
// native-size blocks. A disposable outer peer holds one partial response only.
func TestDownloadPauseBrowserJourney(t *testing.T) {
	if os.Getenv("KINOSAIL_DOWNLOAD_PAUSE_BROWSER") != "1" {
		t.Skip("KINOSAIL_DOWNLOAD_PAUSE_BROWSER is not set")
	}
	const chunk = 8 * 1024 * 1024
	fixture := append(append(bytes.Repeat([]byte{1}, chunk), bytes.Repeat([]byte{2}, chunk)...), bytes.Repeat([]byte{3}, 31)...)
	t.Logf("fictional native-block fixture: bytes=%d SHA-256=%x", len(fixture), sha256.Sum256(fixture))
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Fictional Range Fixture.mp4"), fixture, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), FFmpeg: "/unavailable-download-fixture", FFprobe: "/unavailable-download-fixture"})
	job := prepareDownloadPauseJob(t, handler)
	waitDownloadPausePrepared(t, handler, job, fixture)
	peer := &downloadPausePeer{handler: handler, job: job}
	web := httptest.NewServer(peer)
	t.Cleanup(web.Close)
	project := os.Getenv("KINOSAIL_BROWSER_PROJECT")
	if project == "" {
		project = "chromium"
	}
	specs := []string{"download-pause.spec.ts", "download-pause-ownership.spec.ts"}
	if os.Getenv("KINOSAIL_DOWNLOAD_PAUSE_HIT_TARGETS") == "1" {
		specs = []string{"download-pause-hit-target.spec.ts"}
	}
	arguments := append([]string{"exec", "playwright", "test"}, specs...)
	arguments = append(arguments, "--workers=1", "--project="+project, "--retries=0", "--forbid-only")
	command := exec.CommandContext(t.Context(), "pnpm", arguments...) //nolint:gosec // Fixed native-browser test against an explicitly disposable local Server.
	command.Dir = "../../e2e"
	command.Env = append(os.Environ(), "KINOSAIL_DOWNLOAD_PAUSE_URL="+web.URL, "KINOSAIL_E2E_URL="+web.URL)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
}

func prepareDownloadPauseJob(t *testing.T, handler http.Handler) string {
	t.Helper()
	libraryResponse := httptest.NewRecorder()
	handler.ServeHTTP(libraryResponse, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
	var library struct {
		Items []struct{ ID, Title string }
	}
	if libraryResponse.Code != http.StatusOK || json.Unmarshal(libraryResponse.Body.Bytes(), &library) != nil || len(library.Items) != 1 || library.Items[0].Title != "Fictional Range Fixture" {
		t.Fatal("disposable public Library fixture was not populated")
	}
	prepared := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/items/"+library.Items[0].ID+"/downloads", strings.NewReader(`{"quality":"original"}`))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(prepared, request)
	var job struct {
		ID string `json:"id"`
	}
	if prepared.Code != http.StatusAccepted || json.Unmarshal(prepared.Body.Bytes(), &job) != nil || job.ID == "" {
		t.Fatalf("public original preparation = %d", prepared.Code)
	}
	return job.ID
}

func waitDownloadPausePrepared(t *testing.T, handler http.Handler, job string, fixture []byte) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/downloads/"+job, nil))
		var state struct {
			State        string `json:"state"`
			SHA256       string `json:"sha256"`
			ReadyOffline bool   `json:"readyOffline"`
		}
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &state) != nil {
			t.Fatal("public preparation status unavailable")
		}
		if state.State == "ready" && state.ReadyOffline && state.SHA256 == fmt.Sprintf("%x", sha256.Sum256(fixture)) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("original-quality preparation did not become ready: %s", state.State)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

type downloadPausePeer struct {
	handler          http.Handler
	mutex            sync.Mutex
	job              string
	ranges           []int
	closed, removals int
	generation       int
	held             bool
}

func (peer *downloadPausePeer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/__download-pause" {
		peer.serveStatus(writer, request)
		return
	}
	if request.Method == http.MethodDelete || strings.HasSuffix(request.URL.Path, "/remove") {
		peer.mutex.Lock()
		peer.removals++
		peer.mutex.Unlock()
		http.Error(writer, "Remove is excluded from this fixture", http.StatusMethodNotAllowed)
		return
	}
	if request.URL.Path != "/api/v1/downloads/"+peer.job+"/file" {
		peer.handler.ServeHTTP(writer, request)
		return
	}
	var offset, end int
	if _, err := fmt.Sscanf(request.Header.Get("Range"), "bytes=%d-%d", &offset, &end); err != nil {
		peer.handler.ServeHTTP(writer, request)
		return
	}
	peer.mutex.Lock()
	peer.ranges = append(peer.ranges, offset)
	held := offset == 8*1024*1024 && !peer.held
	if held {
		peer.held = true
	}
	generation := peer.generation
	peer.mutex.Unlock()
	if !held {
		peer.handler.ServeHTTP(writer, request)
		return
	}
	peer.serveHeldRange(writer, request, generation)
}

func (peer *downloadPausePeer) serveHeldRange(writer http.ResponseWriter, request *http.Request, generation int) {
	// Obtain headers and bytes from the real Go range handler, then hold its body.
	response := httptest.NewRecorder()
	peer.handler.ServeHTTP(response, request)
	for name, values := range response.Header() {
		if name != "Content-Length" {
			writer.Header()[name] = values
		}
	}
	writer.WriteHeader(response.Code)
	if _, err := io.CopyN(writer, response.Body, 65536); err != nil {
		return
	}
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
	<-request.Context().Done()
	peer.mutex.Lock()
	if peer.generation == generation {
		peer.closed++
	}
	peer.mutex.Unlock()
}

func (peer *downloadPausePeer) serveStatus(writer http.ResponseWriter, request *http.Request) {
	peer.mutex.Lock()
	defer peer.mutex.Unlock()
	if request.Method == http.MethodPost {
		peer.ranges, peer.closed, peer.removals, peer.held = []int{}, 0, 0, false
		peer.generation++
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(map[string]any{"ranges": peer.ranges, "closed": peer.closed, "removals": peer.removals})
}
