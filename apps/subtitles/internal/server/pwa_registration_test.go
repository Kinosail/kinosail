package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestPWAHandlesServiceWorkerRegistrationFailure(t *testing.T) {
	response := httptest.NewRecorder()
	server.New(server.Config{}).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/pwa.js", nil))
	script := response.Body.String()
	if !strings.Contains(script, `serviceWorker.register("/service-worker.js?v=45").then`) || !strings.Contains(script, `.catch(() => {});`) {
		t.Fatalf("service worker registration failure is unhandled: %q", script)
	}
}

func TestOfflineDownloadClientAndWorkerExposeLocalPlaybackContract(t *testing.T) {
	handler := server.New(server.Config{})
	client := httptest.NewRecorder()
	handler.ServeHTTP(client, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/downloads.js", nil))
	worker := httptest.NewRecorder()
	handler.ServeHTTP(worker, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/service-worker.js", nil))
	if !strings.Contains(worker.Body.String(), `const cacheName = "kinosail-shell-v45"`) || !strings.Contains(worker.Body.String(), `fetch(event.request).catch(() => caches.match("/offline"))`) {
		t.Fatalf("service worker does not refresh versioned assets before shell cache fallback: %q", worker.Body.String())
	}
	for _, value := range []string{"const chunkSize = 8 * 1024 * 1024", "indexedDB", "crypto.subtle.digest", "Range", "Content-Digest", "sha256", "offline-media", "storage?.estimate", "storage.getDirectory", "navigator.locks", "BroadcastChannel", "EventSource"} {
		if !strings.Contains(client.Body.String()+worker.Body.String(), value) {
			t.Fatalf("offline client/worker lacks %q", value)
		}
	}
	if strings.Contains(client.Body.String()+worker.Body.String(), "blob.arrayBuffer()") {
		t.Fatal("offline playback copies the complete media file into memory")
	}
	for _, value := range []string{"job.profileId", "record.profileID", "store.get(jobID)"} {
		if !strings.Contains(client.Body.String()+worker.Body.String(), value) {
			t.Fatalf("offline client/worker lacks profile boundary %q", value)
		}
	}
	if strings.Contains(worker.Body.String(), `cache.put(event.request`) || !strings.Contains(worker.Body.String(), `retiredOfflinePages`) {
		t.Fatalf("service worker retains authenticated player pages: %q", worker.Body.String())
	}
}
