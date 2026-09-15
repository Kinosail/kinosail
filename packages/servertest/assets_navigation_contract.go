package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// AssetsFixture binds asset contracts to real app construction and authenticated web calls.
type AssetsFixture struct {
	NewHandler func(string, string, bool) http.Handler
	SignIn     func(*testing.T, http.Handler, string, string) *http.Cookie
	WebCall    func(*testing.T, http.Handler, string, string, string, *http.Cookie) *httptest.ResponseRecorder
}

// HomeHasSelfHostedFrontendAssets preserves the shared app asset contract.
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

// LibraryNavigationKeepsEveryDestinationInMainWithCompactOverflow preserves the shared app asset contract.
func (fixture AssetsFixture) LibraryNavigationKeepsEveryDestinationInMainWithCompactOverflow(t *testing.T) {
	t.Helper()
	t.Parallel()

	handler := fixture.NewHandler("", "", false)
	home, music, styles := httptest.NewRecorder(), httptest.NewRecorder(), httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	handler.ServeHTTP(music, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=music", nil))
	handler.ServeHTTP(styles, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css", nil))

	markup := home.Body.String()
	for _, expected := range []string{`aria-label="Main navigation"`, `href="/?view=all">Home`, `href="/?view=list">My List`, `href="/?view=movies">Movies`, `href="/?view=shows">Shows`, `class="nav-main-overflow" href="/?view=history">History`, `<details class="nav-more">`, `class="nav-more-section nav-more-library"`, `class="nav-more-section nav-more-actions"`} {
		if !strings.Contains(markup, expected) {
			t.Fatalf("home navigation missing %q: %q", expected, markup)
		}
	}
	if !strings.Contains(music.Body.String(), `<summary class="active">More</summary>`) {
		t.Fatalf("navigation active state is incorrect: home=%q music=%q", markup, music.Body.String())
	}
	for _, expected := range []string{`@media(min-width:901px){.library-page{padding-left:16rem}`, `grid-template:"brand" auto "search" auto "nav"`, `.nav-more{display:none}`, `@media(max-width:900px){.library-page{padding-left:0}`, `grid-template-columns:repeat(5,minmax(0,1fr))`, `.nav-main-overflow,.nav-main-edit{display:none!important}`, `.header-compact-panel{position:fixed`, `.nav-more-menu{position:fixed`, `.header-compact-menu>summary:after{content:none}`, `.app-header nav>a.active{border-color:transparent`, `.app-header nav a[href="/?view=all"]:before`, `.search{right:max(.5rem,env(safe-area-inset-right));left:max(4rem,env(safe-area-inset-left))`, `.search:before{position:absolute`, `.search input:not([type=hidden]){background:var(--overlay)`, `.library-shell{padding-bottom:calc(7.5rem`, `.nav-more-section{display:grid;grid-column:1/-1`, `.nav-more-menu button,.nav-more-menu a{display:flex!important`, `.nav-more-menu button[hidden]{display:none!important`, `.header-actions{display:none!important}`, `.brand-lockup h1{position:absolute;display:block;width:1px;height:1px;overflow:hidden`, `.search,.header-compact-menu>summary{position:fixed;z-index:59;bottom:calc(4.875rem`, `.header-compact-menu>summary{left:max(.5rem,env(safe-area-inset-left));width:48px;height:48px}`, `grid-template:"brand" auto "search" auto "nav" max-content "actions" auto/1fr`, `scrollbar-gutter:stable`, `.app-header nav{align-self:auto;overflow:visible}`} {
		if !strings.Contains(styles.Body.String(), expected) {
			t.Fatalf("responsive navigation styles missing %q", expected)
		}
	}
}

// LibraryExposesDiscoverableCommandsAndInputParity preserves the shared app asset contract.
func (fixture AssetsFixture) LibraryExposesDiscoverableCommandsAndInputParity(t *testing.T) {
	t.Helper()
	t.Parallel()

	handler := fixture.NewHandler("", "", false)
	home, script, styles := httptest.NewRecorder(), httptest.NewRecorder(), httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	handler.ServeHTTP(script, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/main.kinosail.bundle.js", nil))
	handler.ServeHTTP(styles, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css", nil))

	for _, expected := range []string{`data-command-open`, `data-command-shortcut`, `aria-keyshortcuts="Control+K Meta+K"`, `class="header-compact-menu"`, `class="nav-more-section nav-more-actions"`, `data-command-menu`, `role="dialog"`, `data-command-label="Movies"`, `data-command-label="Offline downloads"`, `data-command-label="Sync"`} {
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

// DesktopRailSeparatesUtilitiesFromAccount preserves the shared app asset contract.
func (fixture AssetsFixture) DesktopRailSeparatesUtilitiesFromAccount(t *testing.T) {
	t.Helper()
	t.Parallel()

	handler := fixture.NewHandler("", t.TempDir(), true)
	cookie := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")
	home := fixture.WebCall(t, handler, http.MethodGet, "/", "", cookie)
	styles := httptest.NewRecorder()
	handler.ServeHTTP(styles, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css", nil))

	for _, expected := range []string{`class="header-utility-links owner-utilities"`, `class="header-account" role="group" aria-label="Profile"`, `class="nav-more-section nav-more-account"`, `class="quiet">Sign out`} {
		if !strings.Contains(home.Body.String(), expected) {
			t.Fatalf("home rail missing %q: %q", expected, home.Body.String())
		}
	}
	for _, expected := range []string{`.header-utility-links{display:grid;grid-template-columns:1fr`, `.header-utility-links.owner-utilities{grid-template-columns:repeat(3,minmax(0,1fr))`, `.header-account{display:grid;grid-template-columns:minmax(0,1fr) auto`, `.header-actions .command-open,.header-utility-links,.header-account{grid-column:1/-1}`, `.header-compact-menu{display:contents}`, `.header-compact-panel{display:grid;grid-template-columns:repeat(2,minmax(0,1fr))`, `backdrop-filter:none`, `header-utility-links .header-link:hover`, `border-radius:10px`} {
		if !strings.Contains(styles.Body.String(), expected) {
			t.Fatalf("desktop rail styles missing %q", expected)
		}
	}
}
