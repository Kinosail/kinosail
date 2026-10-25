package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type AssetsFixture struct {
	NewHandler func(string, string, bool) http.Handler
	SignIn     func(*testing.T, http.Handler, string, string) *http.Cookie
	WebCall    func(*testing.T, http.Handler, string, string, string, *http.Cookie) *httptest.ResponseRecorder
}

func (fixture AssetsFixture) HomeHasSelfHostedFrontendAssets(t *testing.T) {
	t.Helper()
	t.Parallel()

	handler := fixture.NewHandler("", "", false)
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/htmx.min.js", nil))

	if strings.Contains(home.Body.String(), "https://") || !strings.Contains(asset.Body.String(), "htmx") {
		t.Fatalf("home = %q, asset = %d", home.Body.String(), asset.Code)
	}
}

func (fixture AssetsFixture) LibraryExposesDiscoverableCommandsAndInputParity(t *testing.T) {
	t.Helper()
	t.Parallel()

	handler := fixture.NewHandler("", "", false)
	home, script, styles := httptest.NewRecorder(), httptest.NewRecorder(), httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	handler.ServeHTTP(script, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/main.kinosail.bundle.js", nil))
	handler.ServeHTTP(styles, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css", nil))

	for _, expected := range []string{`data-command-open`, `data-command-shortcut`, `aria-keyshortcuts="Control+K Meta+K"`, `class="header-compact-menu"`, `class="nav-more-section nav-more-actions"`, `data-command-menu`, `role="dialog"`, `data-command-label="Movies"`, `data-command-label="Sync"`} {
		if !strings.Contains(home.Body.String(), expected) {
			t.Fatalf("home missing %q: %q", expected, home.Body.String())
		}
	}
	for _, expected := range []string{`event.key === "/"`, `event.key.toLowerCase() === "k"`, `navigator.platform`, `Ctrl K`, `querySelectorAll("[data-command-open]")`, `querySelectorAll("[data-install]")`, `matchMedia("(min-width: 901px)")`, `sessionStorage`, `event.persisted`, `ArrowDown`, `ArrowRight`, `preventScroll`} {
		if !strings.Contains(script.Body.String(), expected) {
			t.Fatalf("script missing %q: %q", expected, script.Body.String())
		}
	}
	if strings.Contains(home.Body.String(), `class="header-compact-menu" open`) {
		t.Fatalf("compact Actions menu must not be open before JavaScript: %q", home.Body.String())
	}
	for _, expected := range []string{`.command-menu`, `.command-results`, `.card:focus-visible`} {
		if !strings.Contains(styles.Body.String(), expected) {
			t.Fatalf("styles missing %q: %q", expected, styles.Body.String())
		}
	}
}
