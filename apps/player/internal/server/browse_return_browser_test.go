package server_test

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
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
	specs, valid := browseReturnSpecs(mode)
	if !valid {
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
	media := populateBrowseReturnFixture(t, clip)
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), FFmpeg: "/unavailable-browse-return-fixture", FFprobe: "/unavailable-browse-return-fixture"})
	peer := &browseReturnPeer{handler: handler}
	web := httptest.NewServer(peer)
	t.Cleanup(web.Close)
	project := os.Getenv("KINOSAIL_BROWSER_PROJECT")
	if project == "" {
		project = "chromium"
	}
	arguments := append([]string{"node_modules/@playwright/test/cli.js", "test"}, specs...)
	arguments = append(arguments, "--workers=1", "--retries=0", "--project="+project)
	if os.Getenv("KINOSAIL_BROWSE_RETURN_PROOF") == "1" {
		// Hosted diagnosis publishes bounded fixture observations, never raw logs.
		arguments = append(arguments, "--reporter=./browse-return-proof-reporter.ts", "--trace=off")
	}
	command := exec.CommandContext(t.Context(), "node", arguments...) //nolint:gosec // Fixed installed test CLI and explicitly disposable local Server; no package-manager resolution.
	command.Dir = "../../e2e"
	command.Env = append(os.Environ(), "KINOSAIL_BROWSE_RETURN_URL="+web.URL, "KINOSAIL_BROWSE_RETURN_CASES="+mode, "KINOSAIL_E2E_URL="+web.URL)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
}

func browseReturnSpecs(mode string) ([]string, bool) {
	specs, valid := map[string][]string{
		"navigation": {"watch-navigation.spec.ts"},
		"primary":    {"browse-return.spec.ts"},
		"cold":       {"browse-return-cold.spec.ts"},
		"bfcache":    {"browse-return-bfcache.spec.ts"},
		"htmx":       {"browse-return.spec.ts"},
		"shows":      {"browse-return.spec.ts"},
		"search":     {"browse-return-cold.spec.ts"},
		"safety":     {"browse-return-safety.spec.ts"},
		"home":       {"browse-return-home.spec.ts"},
		"all":        {"browse-return.spec.ts", "browse-return-cold.spec.ts", "browse-return-bfcache.spec.ts"},
	}[mode]
	return specs, valid
}

func populateBrowseReturnFixture(t *testing.T, clip []byte) string {
	t.Helper()
	media := t.TempDir()
	root, err := os.OpenRoot(media)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	write := func(name string, playable bool) {
		t.Helper()
		var data []byte
		if playable {
			data = clip
		}
		if err := writeBrowseReturnFixture(root, name, data); err != nil {
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
	return media
}

func writeBrowseReturnFixture(root *os.Root, name string, data []byte) error {
	if !filepath.IsLocal(name) {
		return errors.New("fixture path must stay within the media root")
	}
	if err := root.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		return err
	}
	return root.WriteFile(name, data, 0o600)
}

// Public browser fixtures only use controlled valid names. This isolated guard
// protects the writer boundary their populated journeys cannot exercise.
func TestBrowseReturnFixtureRejectsEscapingPaths(t *testing.T) {
	t.Parallel()
	media := t.TempDir()
	root, err := os.OpenRoot(media)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, name := range []string{"", "../outside.mp4", "nested/../../outside.mp4", filepath.Join(media, "absolute.mp4")} {
		if err := writeBrowseReturnFixture(root, name, nil); err == nil {
			t.Error("an escaping fixture path was accepted")
		}
	}
	entries, err := os.ReadDir(media)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("rejected fixture paths must not create media entries")
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
