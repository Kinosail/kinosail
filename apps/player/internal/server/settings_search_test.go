package server_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestSettingsExposeSmartSearch(t *testing.T) {
	servertest.SettingsExposeSmartSearch(t, settingsSearchHandler)
}

func TestSettingsBookmarksFollowTheRenderedSectionOrder(t *testing.T) {
	t.Parallel()
	response := httptest.NewRecorder()
	settingsSearchHandler(t.TempDir()).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("settings status = %d", response.Code)
	}
	markup := response.Body.String()
	for _, expected := range []string{`href="#playback">Playback &amp; subtitles`, `href="#appearance">Appearance &amp; language`, `href="#library">Library`, `href="#profiles">Viewer Profiles`, `href="#general">General`, `href="#access">Connections`, `href="#security">Security &amp; sharing`, `href="#settings-integrations">Integrations`, `href="#transcoder">Server tools`, `href="#viewing-imports">Import viewing history`} {
		position := strings.Index(markup, expected)
		if position < 0 {
			t.Fatalf("missing or out-of-order category %q", expected)
		}
		markup = markup[position+len(expected):]
	}
	sections := regexp.MustCompile(`<section[^>]*><h2>[^<]+</h2>`).FindAllString(response.Body.String(), -1)
	expected := map[string]string{"Server name": "general", "Software updates": "general", "Setup guide": "general", "Playback": "playback", "Subtitles": "playback", "Profiles": "household", "Library folders": "library", "Appearance": "appearance", "Video conversion": "system", "API keys": "integrations", "Automatic sign-out": "security", "Trusted HTTPS (Required for Jellyfin apps)": "network", "Move viewing activity": "migration", "Sign-in protection": "security", "Watch away from home": "network", "Library discovery": "library"}
	ids := map[string]bool{}
	idPattern := regexp.MustCompile(` id="([^"]+)"`)
	for _, section := range sections {
		assertSettingsCategories(t, section, expected)
		id := idPattern.FindStringSubmatch(section)
		if len(id) != 2 || ids[id[1]] {
			t.Fatalf("missing or duplicate bookmark: %s", section)
		}
		ids[id[1]] = true
		if strings.Contains(section, " hidden") {
			t.Errorf("settings must remain available without JavaScript: %s", section)
		}
	}
	if len(expected) > 0 {
		t.Errorf("missing settings: %v", expected)
	}
}

func settingsSearchHandler(dataDir string) http.Handler {
	return server.New(server.Config{DataDir: dataDir})
}

func assertSettingsCategories(t *testing.T, section string, expected map[string]string) {
	t.Helper()
	for heading, category := range expected {
		if strings.Contains(section, "<h2>"+heading+"</h2>") {
			if !strings.Contains(section, `data-settings-category="`+category+`"`) {
				t.Errorf("wrong category for %s: %s", heading, section)
			}
			delete(expected, heading)
		}
	}
}
