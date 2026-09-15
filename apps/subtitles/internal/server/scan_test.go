package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestLibraryRefreshesOnSchedule(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: mediaDir, ScanInterval: 5 * time.Millisecond})
	if err := os.WriteFile(filepath.Join(mediaDir, "Dune.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for range 50 {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
		if strings.Contains(response.Body.String(), "Dune") {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("scheduled scan did not find Dune")
}

func TestLibraryAutomaticallyDetectsNewNestedMediaForWebAndAPI(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: mediaDir, ScanInterval: time.Hour})
	time.Sleep(100 * time.Millisecond) // Let real-time monitoring finish its startup reconciliation.
	season := filepath.Join(mediaDir, "Severance", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(season, "Severance.S01E01.Good.News.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		web := httptest.NewRecorder()
		handler.ServeHTTP(web, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
		api := httptest.NewRecorder()
		handler.ServeHTTP(api, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
		if strings.Contains(web.Body.String(), "Severance") && strings.Contains(api.Body.String(), `"title":"S01E01 · Good News"`) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("filesystem change was not visible through the web and API Library adapters")
}

func TestLibraryMonitoringWorksWithoutScheduledScans(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: mediaDir})
	time.Sleep(100 * time.Millisecond) // Let real-time monitoring register the root.
	if err := os.WriteFile(filepath.Join(mediaDir, "Dune.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}

	assertLibraryEventuallyContains(t, handler, "Dune", 8*time.Second)
}

func TestLibraryWaitsForCopyToSettle(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: mediaDir, ScanInterval: time.Hour})
	time.Sleep(100 * time.Millisecond) // Let real-time monitoring register the root.
	file, err := os.Create(filepath.Join(mediaDir, "Arrival.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString("first chunk"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2500 * time.Millisecond)
	if response := libraryPage(t, handler); strings.Contains(response.Body.String(), "Arrival") {
		t.Fatalf("growing file was published: %s", response.Body.String())
	}
	if _, err := file.WriteString("final chunk"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	assertLibraryEventuallyContains(t, handler, "Arrival", 8*time.Second)
}

func TestFailedScanKeepsLastKnownGoodLibrary(t *testing.T) {
	t.Parallel()

	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Heat.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir})
	assertLibraryEventuallyContains(t, handler, "Heat", 5*time.Second)
	if err := os.RemoveAll(mediaDir); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/tasks/scan", nil))
	home := libraryPage(t, handler)

	if response.Code != http.StatusBadGateway || !strings.Contains(home.Body.String(), "Heat") {
		t.Fatalf("scan = %d, home = %q", response.Code, home.Body.String())
	}
}

func TestOwnerCanDisableSafetyScansWithoutDisablingAutoDetection(t *testing.T) {
	t.Parallel()

	mediaDir, dataDir := t.TempDir(), t.TempDir()
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: mediaDir, DataDir: dataDir, ScanInterval: 5 * time.Millisecond})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/scans", strings.NewReader("frequency=off"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if err := os.WriteFile(filepath.Join(mediaDir, "Dune.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(8 * time.Second)
	detected := false
	for time.Now().Before(deadline) {
		home := httptest.NewRecorder()
		handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
		if strings.Contains(home.Body.String(), "Dune") {
			detected = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	handler = server.New(server.Config{Lifecycle: t.Context(), MediaDir: mediaDir, DataDir: dataDir, ScanInterval: 5 * time.Millisecond})
	home, response := httptest.NewRecorder(), httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))

	if !detected || !strings.Contains(home.Body.String(), "Dune") || !strings.Contains(response.Body.String(), `<option value="off" selected>Off`) {
		t.Fatalf("home = %q, settings = %q", home.Body.String(), response.Body.String())
	}
}

func assertLibraryEventuallyContains(t *testing.T, handler http.Handler, title string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if response := libraryPage(t, handler); strings.Contains(response.Body.String(), title) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("Library did not discover %s", title)
}

func libraryPage(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	return response
}
