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

func TestServerWithoutLifecycleDoesNotStartBackgroundScans(t *testing.T) {
	t.Parallel()
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Arrival.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: media, ScanInterval: time.Millisecond})
	if err := os.WriteFile(filepath.Join(media, "Primer.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if strings.Contains(response.Body.String(), "Primer") {
		t.Fatal("Server started background work without an explicit lifecycle")
	}
}

func TestServerWithoutLifecycleDoesNotStartBackgroundMaintenance(t *testing.T) {
	t.Parallel()
	cache := t.TempDir()
	transcode := filepath.Join(cache, "0000000000000001")
	if err := os.Mkdir(transcode, 0o700); err != nil {
		t.Fatal(err)
	}
	segment := filepath.Join(transcode, "segment.m4s")
	if err := os.WriteFile(segment, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = server.New(server.Config{CacheDir: cache, MaintenanceInterval: time.Millisecond, TranscodeCacheLimit: 1})
	time.Sleep(20 * time.Millisecond)
	if _, err := os.Stat(segment); err != nil {
		t.Fatalf("Server started background maintenance without an explicit lifecycle: %v", err)
	}
}

func TestFailedStartupStopsBackgroundMaintenance(t *testing.T) {
	t.Parallel()
	cache := t.TempDir()
	transcode := filepath.Join(cache, "0000000000000001")
	if err := os.Mkdir(transcode, 0o700); err != nil {
		t.Fatal(err)
	}
	segment := filepath.Join(transcode, "segment.m4s")
	if err := os.WriteFile(segment, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	downloads := filepath.Join(cache, "downloads")
	if err := os.Mkdir(downloads, 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(downloads, "aaaaaaaaaaaaaaaa.json")
	if err := os.WriteFile(manifest, []byte(`{"id":"aaaaaaaaaaaaaaaa","extension":"../../outside"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{Lifecycle: t.Context(), DataDir: t.TempDir(), CacheDir: cache, MaintenanceInterval: time.Second, TranscodeCacheLimit: 1})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("invalid startup = %d %q", response.Code, response.Body.String())
	}
	time.Sleep(1200 * time.Millisecond)
	if _, err := os.Stat(segment); err != nil {
		t.Fatalf("failed application kept background maintenance running: %v", err)
	}
}

func TestHealthyStartupKeepsBackgroundMaintenance(t *testing.T) {
	t.Parallel()
	cache := t.TempDir()
	transcode := filepath.Join(cache, "0000000000000001")
	if err := os.Mkdir(transcode, 0o700); err != nil {
		t.Fatal(err)
	}
	segment := filepath.Join(transcode, "segment.m4s")
	if err := os.WriteFile(segment, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{Lifecycle: t.Context(), DataDir: t.TempDir(), CacheDir: cache, MaintenanceInterval: 100 * time.Millisecond, TranscodeCacheLimit: 1})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("healthy startup = %d %q", response.Code, response.Body.String())
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(segment); os.IsNotExist(err) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("healthy application did not start background maintenance")
}
