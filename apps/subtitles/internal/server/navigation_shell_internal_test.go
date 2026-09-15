package server

import "testing"

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
