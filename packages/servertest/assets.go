// Package servertest runs shared browser-server contracts against each app.
package servertest

import (
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// AssertInstallableWebApp verifies the common PWA resources exposed by an app.
func AssertInstallableWebApp(t *testing.T, handler http.Handler) {
	t.Helper()
	home := request(t, handler, "/")
	manifest := request(t, handler, "/manifest.webmanifest")
	icon := request(t, handler, "/static/icon.svg")
	favicon := request(t, handler, "/favicon.ico")
	worker := request(t, handler, "/service-worker.js")
	if manifest.Header().Get("Content-Type") != "application/manifest+json" || icon.Header().Get("Content-Type") != "image/svg+xml" || strings.Contains(icon.Body.String(), "linearGradient") || favicon.Code != http.StatusOK || favicon.Header().Get("Content-Type") != "image/svg+xml" || worker.Header().Get("Service-Worker-Allowed") != "/" || worker.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("home = %q, manifest = %d %q, icon = %d", home.Body.String(), manifest.Code, manifest.Body.String(), icon.Code)
	}
	assertContains(t, "home", home.Body.String(), `rel="manifest"`)
	assertContains(t, "manifest", manifest.Body.String(), `"id":"/"`, `"scope":"/"`, `"display":"standalone"`, `"theme_color":"#0b0d0b"`, `"sizes":"192x192"`, `"sizes":"512x512"`)
	assertContains(t, "icon", icon.Body.String(), "#c4ff47")
	assertContains(t, "service worker", worker.Body.String(), "/static/cinema-backdrop.jpg")
	assertWebAppIcons(t, handler)
}

func assertWebAppIcons(t *testing.T, handler http.Handler) {
	t.Helper()
	for path, size := range map[string]int{"/static/icon-192.png": 192, "/static/icon-512.png": 512, "/static/icon-maskable-512.png": 512, "/static/apple-touch-icon.png": 180} {
		response := request(t, handler, path)
		image, err := png.Decode(response.Body)
		if err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		if response.Header().Get("Content-Type") != "image/png" || image.Bounds().Dx() != size || image.Bounds().Dy() != size {
			t.Fatalf("%s = %d %q, bounds = %v", path, response.Code, response.Header().Get("Content-Type"), image.Bounds())
		}
	}
}

// AssertBrandLogo verifies the app-specific logo rendered by shared surfaces.
func AssertBrandLogo(t *testing.T, handler http.Handler, setupFragments, iconFragments, styleFragments, forbiddenIconFragments []string) {
	t.Helper()
	setup := request(t, handler, "/setup")
	icon := request(t, handler, "/static/icon.svg")
	styles := request(t, handler, "/static/app.css")
	assertContains(t, "setup", setup.Body.String(), setupFragments...)
	assertContains(t, "icon", icon.Body.String(), iconFragments...)
	assertContains(t, "styles", styles.Body.String(), styleFragments...)
	for _, fragment := range forbiddenIconFragments {
		if strings.Contains(icon.Body.String(), fragment) {
			t.Fatalf("icon unexpectedly contains %q: %q", fragment, icon.Body.String())
		}
	}
}

func request(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	return response
}

func assertContains(t *testing.T, name, body string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(body, fragment) {
			t.Fatalf("%s missing %q: %q", name, fragment, body)
		}
	}
}
