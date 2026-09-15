package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ViewerCanChooseDarkLightOrSystemTheme preserves the shared app asset contract.
func (fixture AssetsFixture) ViewerCanChooseDarkLightOrSystemTheme(t *testing.T) {
	t.Helper()
	t.Parallel()

	handler := fixture.NewHandler("", "", false)
	home, settings, script, styles := httptest.NewRecorder(), httptest.NewRecorder(), httptest.NewRecorder(), httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	handler.ServeHTTP(script, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/theme.js", nil))
	handler.ServeHTTP(styles, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css", nil))

	page := settings.Body.String()
	for _, expected := range []string{`<legend>Theme</legend>`, `name="theme" value="dark" data-theme-choice checked`, `name="theme" value="light" data-theme-choice`, `name="theme" value="system" data-theme-choice`, `/static/theme.js`} {
		if !strings.Contains(page, expected) {
			t.Fatalf("settings missing %q: %q", expected, page)
		}
	}
	if strings.Contains(home.Body.String(), `data-theme-choice`) {
		t.Fatalf("home should leave theme selection in settings: %q", home.Body.String())
	}
	for _, expected := range []string{`const themes = ["dark", "light", "system"]`, `localStorage.getItem(themeKey) || "dark"`, `catch { theme = "dark"; }`, `if (!themes.includes(theme)) theme = "dark"`, `localStorage.setItem(themeKey, theme)`, `prefers-color-scheme: dark`, `document.documentElement.dataset.theme`, `systemTheme.addEventListener`} {
		if !strings.Contains(script.Body.String(), expected) {
			t.Fatalf("theme script missing %q: %q", expected, script.Body.String())
		}
	}
	if script.Header().Get("Content-Type") != "text/javascript; charset=utf-8" || !strings.Contains(styles.Body.String(), `:root[data-theme=light]`) || !strings.Contains(styles.Body.String(), `@media(prefers-color-scheme:light)`) {
		t.Fatalf("script = %d %q, styles = %q", script.Code, script.Header().Get("Content-Type"), styles.Body.String())
	}
}

// SettingsAndSharedPageFamiliesUseSignalLayout preserves the shared app asset contract.
func (fixture AssetsFixture) SettingsAndSharedPageFamiliesUseSignalLayout(t *testing.T) {
	t.Helper()
	t.Parallel()

	handler := fixture.NewHandler("", "", false)
	settings, configuration, styles := httptest.NewRecorder(), httptest.NewRecorder(), httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	handler.ServeHTTP(configuration, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings/configuration", nil))
	handler.ServeHTTP(styles, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css", nil))

	for _, expected := range []string{`data-settings-nav`, `data-settings-flow`, `data-settings-group="general"`, `href="#security"`, `href="#playback"`, `href="#access"`, `href="#library"`, `href="#appearance"`, `id="appearance"`, `data-theme-choice`} {
		if !strings.Contains(settings.Body.String(), expected) {
			t.Fatalf("settings missing %q: %q", expected, settings.Body.String())
		}
	}
	if strings.Contains(settings.Body.String(), `<section hidden`) {
		t.Fatal("settings capabilities must remain visible without JavaScript")
	}
	if !strings.Contains(configuration.Body.String(), `Using the Kinosail default.`) {
		t.Fatalf("configuration missing default source: %q", configuration.Body.String())
	}
	for _, expected := range []string{`.settings-shell{display:grid`, `.settings-flow{display:grid;grid-template-columns:1fr`, `.settings-flow{grid-template-columns:minmax(0,1fr);gap:0;max-width:60rem}`, `input[type=checkbox],input[type=radio]`, `.detail-shell{width:`, `.episode{display:grid`, `.player-actions{display:flex`, `.curation-options{display:grid;max-height:min(28rem,60vh)`, `.curation-options section:only-child{grid-column:1/-1}`, `.curation-options button{display:grid;width:100%;min-height:44px`, `.card:has(> form),.card:has(> .playlist-order){display:grid;grid-template-rows:minmax(0,1fr) auto`, `.book-reader{width:100%`} {
		if !strings.Contains(styles.Body.String(), expected) {
			t.Fatalf("styles missing %q: %q", expected, styles.Body.String())
		}
	}
}
