// Package servertest shares regression contracts between real app adapters.
package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// SettingsExposeSmartSearch runs the corresponding app regression contract.
func SettingsExposeSmartSearch(t *testing.T, newHandler func(string) http.Handler) {
	t.Parallel()
	handler := newHandler("")
	settings, script, styles := httptest.NewRecorder(), httptest.NewRecorder(), httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	handler.ServeHTTP(script, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/main.kinosail.bundle.js", nil))
	handler.ServeHTTP(styles, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css", nil))
	for _, expected := range []string{`data-settings-search`, `data-settings-search-input`, `Search by name or task`, `data-settings-search-results`} {
		if !strings.Contains(settings.Body.String(), expected) {
			t.Fatalf("settings missing %q", expected)
		}
	}
	for _, expected := range []string{`searchableText`, `settings-search-result-group`, `No settings match that search.`, `document.createElement("a")`, `#library-search, #settings-search-input`} {
		if !strings.Contains(script.Body.String(), expected) {
			t.Fatalf("settings script missing %q", expected)
		}
	}
	for _, expected := range []string{`.settings-search`, `.settings-search-results`, `.settings-search-result-group`} {
		if !strings.Contains(styles.Body.String(), expected) {
			t.Fatalf("settings styles missing %q", expected)
		}
	}
}

// SettingsBookmarksFollowTheRenderedSectionOrder runs the corresponding app regression contract.
func SettingsBookmarksFollowTheRenderedSectionOrder(t *testing.T, newHandler func(string) http.Handler) {
	t.Parallel()
	handler := newHandler(t.TempDir())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	markup := response.Body.String()
	for _, expected := range []string{
		`href="#playback">Playback`,
		`href="#access">Access`,
		`href="#library">Library`,
		`href="#security">General`,
		`href="#appearance">Appearance`,
		`href="#system">System`,
		`href="#viewing-imports">Migration`,
	} {
		position := strings.Index(markup, expected)
		if position < 0 {
			t.Fatalf("settings bookmark missing %q", expected)
		}
		markup = markup[position+len(expected):]
	}

	markup = response.Body.String()
	for _, expected := range []string{`id="playback" data-settings-group="playback"`, `id="access" data-settings-group="access"`, `id="library" data-settings-group="library"`, `id="security" data-settings-group="general"`, `id="appearance" data-settings-group="appearance"`, `id="system" data-settings-group="system"`, `id="viewing-imports" data-settings-group="migration"`} {
		position := strings.Index(markup, expected)
		if position < 0 {
			t.Fatalf("settings section missing %q", expected)
		}
		markup = markup[position+len(expected):]
	}
}
