package server

import (
	"bytes"
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
	for _, version := range []string{"72", "80", "81", "82", "83", "84", "85", "91", "93", "94", "impeccable-1", "electric-1", "electric-4", "electric-18"} {
		page := []byte(`<link rel="stylesheet" href="/static/app.css?v=` + version + `">`)
		want := []byte(`<link rel="stylesheet" href="/static/app.css?v=electric-28">`)
		if actual := applicationShellCSSVersion(page); !bytes.Equal(actual, want) {
			t.Fatalf("stylesheet %s was not refreshed: %s", version, actual)
		}
	}
}

func TestApplicationShellLoadsRecognitionOnce(t *testing.T) {
	for _, existing := range []string{"", `<script defer src="/static/supporter.js?v=17"></script>`} {
		page := injectApplicationShell([]byte(`<html><body><main>Account</main>`+existing+`</body></html>`), nil)
		if bytes.Count(page, []byte(`/static/supporter.js?v=17`)) != 1 {
			t.Fatalf("recognition script missing or duplicated: %s", page)
		}
	}
}
