package server

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApplicationShellRouteBoundary(t *testing.T) {
	for _, pattern := range []string{
		"GET /account",
		"GET /album/{id}",
		"GET /book/{id}",
		"GET /collection/{name}",
		"GET /metadata/bulk",
		"GET /offline-downloads",
		"GET /playlist/{name}",
		"GET /quick-connect",
		"GET /settings",
		"GET /settings/agent-connections",
		"GET /settings/backups",
		"GET /settings/configuration",
		"GET /settings/media-shares",
		"GET /settings/remote-readiness",
		"GET /settings/management",
		"GET /settings/system",
		"GET /show/{id}",
		"GET /supporter",
	} {
		if !applicationShellRoute(pattern) {
			t.Errorf("application shell excludes %q", pattern)
		}
	}

	for _, pattern := range []string{
		"GET /{$}",
		"GET /login",
		"GET /oauth/authorize",
		"GET /onboarding",
		"GET /read/{id}",
		"GET /setup",
		"GET /share",
		"GET /watch-together/{id}",
		"GET /watch/{id}",
		"POST /settings",
	} {
		if applicationShellRoute(pattern) {
			t.Errorf("application shell includes focused route %q", pattern)
		}
	}
}

func TestApplicationShellRefreshesCachedStylesheetVersions(t *testing.T) {
	response := httptest.NewRecorder()
	serveStyle(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css", nil))
	current := fmt.Sprintf("%x", sha256.Sum256(response.Body.Bytes()))
	for _, version := range []string{"72", "80", "81", "82", "83", "84", "85", "91", "93", "94", "impeccable-1", "electric-1", "electric-4", "electric-18"} {
		page := []byte(`<link rel="stylesheet" href="/static/app.css?v=` + version + `">`)
		want := []byte(`<link rel="stylesheet" href="/static/app.css?v=` + current + `">`)
		if actual := applicationShellCSSVersion(page); !bytes.Equal(actual, want) {
			t.Fatalf("stylesheet %s was not refreshed: %s", version, actual)
		}
	}
}

func TestApplicationShellLoadsRecognitionOnce(t *testing.T) {
	for _, existing := range []string{"", `<script defer src="/static/supporter.js?v=19-htmx4"></script>`} {
		page := injectApplicationShell([]byte(`<html><body><main>Account</main>`+existing+`</body></html>`), nil)
		if bytes.Count(page, []byte(`/static/supporter.js?v=19-htmx4`)) != 1 {
			t.Fatalf("recognition script missing or duplicated: %s", page)
		}
	}
}

func TestApplicationShellMainTarget(t *testing.T) {
	// Attribute suffixes must not hide the missing target; existing IDs must survive.
	for _, main := range []string{`<main>`, `<main data-palette-id="art">`, `<main class="detail-shell" data-palette-id="art">`, `<main id="main" data-palette-id="art">`} {
		page := injectApplicationShell([]byte(`<html><body>`+main+`<section id="seasons">Episodes</section></main></body></html>`), nil)
		if bytes.Count(page, []byte(` id="main"`)) != 1 || !bytes.Contains(page, []byte(`href="#main"`)) {
			t.Errorf("main target missing or duplicated for %s: %s", main, page)
		}
	}
}
