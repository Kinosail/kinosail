package servertest

import (
	"net/http"
	"strings"
	"testing"
)

// AssertEveryMainNavigationDestinationRetainsTheMainBar retains the original navigation regression.
func AssertEveryMainNavigationDestinationRetainsTheMainBar(t *testing.T, fixture NavigationFixture) {
	handler := fixture.Library.NewHandler("", t.TempDir(), false)
	for _, path := range []string{"/?view=all", "/?view=list", "/?view=movies", "/?view=shows", "/?view=music", "/?view=audiobooks", "/?view=books", "/?view=photos", "/?view=collections", "/?view=playlists", "/?view=unwatched", "/?view=history"} {
		page := APICall(t, handler, "", http.MethodGet, path, nil)
		if page.Code != http.StatusOK || MainNavigationMarkup(page.Body.String()) == "" {
			t.Errorf("main navigation destination %q = %d, navigation %q", path, page.Code, MainNavigationMarkup(page.Body.String()))
		}
	}
}

// AssertSignedInApplicationPagesRetainTheMainBar retains the original navigation regression.
func AssertSignedInApplicationPagesRetainTheMainBar(t *testing.T, fixture NavigationFixture) {
	handler := fixture.Library.NewHandler("", t.TempDir(), false)
	for _, path := range []string{"/account", "/metadata/bulk", "/offline-downloads", "/quick-connect", "/settings", "/settings/agent-connections", "/settings/backups", "/settings/configuration", "/settings/media-shares", "/settings/remote-readiness", "/settings/system", "/supporter"} {
		page := APICall(t, handler, "", http.MethodGet, path, nil)
		if page.Code != http.StatusOK || MainNavigationMarkup(page.Body.String()) == "" || !strings.Contains(page.Body.String(), `class="library-page`) || !strings.Contains(page.Body.String(), `>Browse library</a>`) || !strings.Contains(page.Body.String(), fixture.RequiredCSS) || (fixture.ForbiddenCSS != "" && strings.Contains(page.Body.String(), fixture.ForbiddenCSS)) || strings.Contains(page.Body.String(), `data-command-open`) {
			t.Errorf("application page %q = %d, navigation %q", path, page.Code, MainNavigationMarkup(page.Body.String()))
		}
	}
}
