package server_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestPWAHandlesServiceWorkerRegistrationFailure(t *testing.T) {
	response := httptest.NewRecorder()
	server.New(server.Config{}).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/pwa.js", nil))
	script := response.Body.String()
	if !strings.Contains(script, `serviceWorker.register("/service-worker.js?v=53").then`) || !strings.Contains(script, `.catch(() => {});`) {
		t.Fatalf("service worker registration failure is unhandled: %q", script)
	}
}

func TestOfflineShellLocalizesDynamicStorageStates(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/offline", nil)
	request.AddCookie(&http.Cookie{Name: "kinosail_language", Value: "es"}) //nolint:gosec // G124: this request cookie selects a language; it is not an authentication credential.
	server.New(server.Config{}).ServeHTTP(response, request)
	for _, expected := range []string{`lang="es"`, `data-offline-storage-unsupported="Este navegador no puede almacenar contenido sin conexión de forma segura"`, `data-offline-no-ready="No hay descargas verificadas listas para este Perfil de espectador."`, `data-offline-select-profile="Abre Kinosail con conexión y elige primero un Perfil de espectador."`} {
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("localized offline shell lacks %q: %d %q", expected, response.Code, response.Body.String())
		}
	}
}

func TestOfflineDownloadClientAndWorkerExposeLocalPlaybackContract(t *testing.T) {
	handler := server.New(server.Config{})
	pwa := httptest.NewRecorder()
	handler.ServeHTTP(pwa, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/pwa.js", nil))
	client := httptest.NewRecorder()
	handler.ServeHTTP(client, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/downloads.js", nil))
	worker := httptest.NewRecorder()
	handler.ServeHTTP(worker, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/service-worker.js", nil))
	assertOfflineAssetVersions(t, pwa, client, worker)
	assertOfflineShell(t, worker)
	assertIndexedOfflineChunks(t, client, worker)
	assertBoundedOfflineMedia(t, client, worker)
	assertOfflineProfileIsolation(t, client, worker)
}

func assertOfflineAssetVersions(t *testing.T, pwa, client, worker *httptest.ResponseRecorder) {
	t.Helper()
	workerVersion := regexp.MustCompile(`kinosail-shell-v(\d+)`).FindStringSubmatch(worker.Body.String())
	pwaVersion := regexp.MustCompile(`service-worker\.js\?v=(\d+)`).FindStringSubmatch(pwa.Body.String())
	clientVersion := regexp.MustCompile(`service-worker\.js\?v=(\d+)`).FindStringSubmatch(client.Body.String())
	if len(workerVersion) != 2 || len(pwaVersion) != 2 || len(clientVersion) != 2 || workerVersion[1] != pwaVersion[1] || workerVersion[1] != clientVersion[1] {
		t.Fatalf("service worker versions differ: worker=%v pwa=%v downloads=%v", workerVersion, pwaVersion, clientVersion)
	}
}

func assertOfflineShell(t *testing.T, worker *httptest.ResponseRecorder) {
	t.Helper()
	if !strings.Contains(worker.Body.String(), `const cacheName = "kinosail-shell-v53"`) || !strings.Contains(worker.Body.String(), `event.respondWith(navigation.catch(() => caches.open(cacheName).then((cache) => cache.match("/offline"))))`) || !strings.Contains(worker.Body.String(), `response.ok ? refreshOffline()`) {
		t.Fatalf("service worker does not refresh versioned assets before shell cache fallback: %q", worker.Body.String())
	}
	for _, value := range []string{`"/static/main.kinosail.bundle.js": "text/javascript"`, `credentials: path === "/offline" ? "same-origin" : "omit"`, `cache: "reload"`, `response.status !== 200`, `response.redirected`, `responseURL.origin !== self.location.origin`, `responseURL.pathname !== path`, `responseType !== expectedType`, `else if (url.pathname in shell)`, `caches.open(cacheName).then((cache) => cache.match("/offline"))`, `imageCachePrefix`, `message.type === "profile"`, `event.origin !== self.location.origin`, `privateImagePath`, `cache.put(request, response.clone())`} {
		if !strings.Contains(worker.Body.String(), value) {
			t.Fatalf("service worker lacks complete offline shell behavior %q: %q", value, worker.Body.String())
		}
	}
}

func assertIndexedOfflineChunks(t *testing.T, client, worker *httptest.ResponseRecorder) {
	t.Helper()
	for name, script := range map[string]string{"client": client.Body.String(), "worker": worker.Body.String()} {
		for _, value := range []string{`indexedDB.open(offlineDatabase, 4)`, `chunks.createIndex("jobRange", ["jobID", "offset"])`} {
			if !strings.Contains(script, value) {
				t.Fatalf("offline %s lacks indexed chunk selection %q: %q", name, value, script)
			}
		}
	}
	if !strings.Contains(worker.Body.String(), `.index("jobRange")`) {
		t.Fatalf("offline worker lacks indexed range selection: %q", worker.Body.String())
	}
	if strings.Contains(worker.Body.String(), `event.request.destination !== "style"`) || strings.Contains(worker.Body.String(), `catch {}`) || strings.Contains(worker.Body.String(), `caches.match(`) {
		t.Fatalf("service worker can install an incomplete or unstyled shell: %q", worker.Body.String())
	}
}

func assertBoundedOfflineMedia(t *testing.T, client, worker *httptest.ResponseRecorder) {
	t.Helper()
	for _, value := range []string{"const chunkSize = 8 * 1024 * 1024", "indexedDB", "crypto.subtle.digest", "Range", "Content-Digest", "sha256", "offline-media", "storage?.estimate", "storage.getDirectory", "navigator.locks", "BroadcastChannel", "EventSource"} {
		if !strings.Contains(client.Body.String()+worker.Body.String(), value) {
			t.Fatalf("offline client/worker lacks %q", value)
		}
	}
	if strings.Contains(client.Body.String()+worker.Body.String(), "blob.arrayBuffer()") {
		t.Fatal("offline playback copies the complete media file into memory")
	}
	if strings.Contains(worker.Body.String(), `.getAll(`) || strings.Contains(worker.Body.String(), `new Blob(parts)`) || !strings.Contains(worker.Body.String(), `new ReadableStream(`) || !strings.Contains(worker.Body.String(), `file.slice(offset, offset + length).arrayBuffer()`) {
		t.Fatal("offline playback does not stream indexed chunks with bounded memory")
	}
	if strings.Contains(worker.Body.String(), `store.getAll())).filter((chunk) => chunk.jobID === job.id)`) || strings.Contains(client.Body.String(), `transaction.objectStore("chunks").getAll()`) {
		t.Fatal("offline storage scans chunks for unrelated jobs")
	}
}

func assertOfflineProfileIsolation(t *testing.T, client, worker *httptest.ResponseRecorder) {
	t.Helper()
	for _, value := range []string{"job.profileId", "record.profileID", "store.get(jobID)", "job.profileID !== profileID"} {
		if !strings.Contains(client.Body.String()+worker.Body.String(), value) {
			t.Fatalf("offline client/worker lacks profile boundary %q", value)
		}
	}
	if strings.Contains(worker.Body.String(), `cache.put(event.request`) || !strings.Contains(worker.Body.String(), `retiredOfflinePages`) {
		t.Fatalf("service worker retains authenticated player pages: %q", worker.Body.String())
	}
}

func TestOfflinePagesUseTheCurrentNavigationBundle(t *testing.T) {
	handler := server.New(server.Config{DataDir: t.TempDir()})
	for _, path := range []string{"/offline-downloads", "/offline"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		body := response.Body.String()
		if !strings.Contains(body, `downloads.js?v=29`) || strings.Contains(body, `main.kinosail.bundle.js?v=12`) || strings.Contains(body, `main.kinosail.bundle.js?v=16`) {
			t.Fatalf("%s can register an obsolete offline worker: %s", path, body)
		}
		if path == "/offline-downloads" && !strings.Contains(body, `main.kinosail.bundle.js?v=30`) {
			t.Fatal("downloads page did not receive the current injected bundle")
		}
	}
}

func TestNavigationPagesRefreshCachedAssets(t *testing.T) {
	handler := server.New(server.Config{DataDir: t.TempDir()})
	for _, path := range []string{"/", "/settings"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		body := response.Body.String()
		if response.Code != http.StatusOK || !strings.Contains(body, `main.kinosail.bundle.js?v=30`) || !strings.Contains(body, `app.css?v=electric-29`) {
			t.Fatalf("%s did not receive current navigation assets", path)
		}
	}
}
