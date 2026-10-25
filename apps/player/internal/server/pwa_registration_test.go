package server_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestOfflineShellLocalizesReconnectPage(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/offline", nil)
	request.AddCookie(&http.Cookie{Name: "kinosail_language", Value: "es"}) //nolint:gosec // G124: this request cookie selects a language; it is not an authentication credential.
	server.New(server.Config{}).ServeHTTP(response, request)
	body := response.Body.String()
	for _, expected := range []string{`lang="es"`, `>Sin conexión<`, `>Estás sin conexión<`, `>Vuelve a conectarte a tu Servidor para seguir viendo.<`, `href="/">Inténtalo de nuevo<`} {
		if response.Code != http.StatusOK || !strings.Contains(body, expected) {
			t.Fatalf("localized offline shell lacks %q: %d %q", expected, response.Code, body)
		}
	}
	if strings.Contains(body, "downloads") || strings.Contains(body, "data-offline-") {
		t.Fatalf("offline shell still offers browser downloads: %q", body)
	}
}

func TestServiceWorkerRetiresBrowserDownloads(t *testing.T) {
	handler := server.New(server.Config{})
	pwa := httptest.NewRecorder()
	handler.ServeHTTP(pwa, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/pwa.js", nil))
	worker := httptest.NewRecorder()
	handler.ServeHTTP(worker, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/service-worker.js", nil))
	client := httptest.NewRecorder()
	handler.ServeHTTP(client, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/downloads.js", nil))
	if client.Code != http.StatusNotFound {
		t.Fatalf("retired downloads client = %d", client.Code)
	}
	workerVersion := regexp.MustCompile(`kinosail-shell-v(\d+)`).FindStringSubmatch(worker.Body.String())
	pwaVersion := regexp.MustCompile(`service-worker\.js\?v=(\d+)`).FindStringSubmatch(pwa.Body.String())
	if len(workerVersion) != 2 || len(pwaVersion) != 2 || workerVersion[1] != pwaVersion[1] {
		t.Fatalf("service worker versions differ: worker=%v pwa=%v", workerVersion, pwaVersion)
	}
	assertOfflineShell(t, worker)
	script := worker.Body.String()
	for _, value := range []string{`}, removeOfflineDownloads);`, `Promise.all([retire(), caches.keys()`, `transaction.objectStore("jobs").clear()`, `transaction.objectStore("chunks").clear()`, `/^[0-9a-f]{16}$/.test(name)`} {
		if !strings.Contains(script, value) {
			t.Fatalf("service worker does not remove retired browser downloads %q", value)
		}
	}
	for _, retired := range []string{"/offline-media/", "downloads.js", "chunkSize", `"/static/player.js"`} {
		if strings.Contains(script, retired) {
			t.Fatalf("service worker still serves browser downloads %q", retired)
		}
	}
	if strings.Contains(script, `cache.put(event.request`) || !strings.Contains(script, `retiredOfflinePages`) {
		t.Fatalf("service worker retains authenticated player pages: %q", script)
	}
}

func assertOfflineShell(t *testing.T, worker *httptest.ResponseRecorder) {
	t.Helper()
	if !strings.Contains(worker.Body.String(), `const cacheName = "kinosail-shell-v56"`) || !strings.Contains(worker.Body.String(), `event.respondWith(navigation.catch(() => caches.open(cacheName).then((cache) => cache.match("/offline"))))`) || !strings.Contains(worker.Body.String(), `response.ok ? refreshOffline()`) {
		t.Fatalf("service worker does not refresh versioned assets before shell cache fallback: %q", worker.Body.String())
	}
	for _, value := range []string{`"/static/main.kinosail.bundle.js": "text/javascript"`, `credentials: path === "/offline" ? "same-origin" : "omit"`, `cache: "reload"`, `response.status !== 200`, `response.redirected`, `responseURL.origin !== self.location.origin`, `responseURL.pathname !== path`, `responseType !== expectedType`, `else if (url.pathname in shell)`, `caches.open(cacheName).then((cache) => cache.match("/offline"))`, `imageCachePrefix`, `message.type === "profile"`, `event.origin !== self.location.origin`, `privateImagePath`, `cache.put(request, response)`, `event.waitUntil(storePrivateImage(cache, event.request, response.clone())`} {
		if !strings.Contains(worker.Body.String(), value) {
			t.Fatalf("service worker lacks complete offline shell behavior %q: %q", value, worker.Body.String())
		}
	}
}
