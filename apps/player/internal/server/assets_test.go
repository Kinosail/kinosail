package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

var assetContracts = servertest.AssetsFixture{
	NewHandler: func(media, data string, auth bool) http.Handler {
		return server.New(server.Config{MediaDir: media, DataDir: data, RequireAuth: auth})
	},
	SignIn: signInTestProfile, WebCall: requestWithCookie,
}

func TestHomeHasSelfHostedFrontendAssets(t *testing.T) {
	assetContracts.HomeHasSelfHostedFrontendAssets(t)
}

func TestLibraryNavigationKeepsEveryDestinationInMainWithCompactOverflow(t *testing.T) {
	assetContracts.LibraryNavigationKeepsEveryDestinationInMainWithCompactOverflow(t)
}

func TestPrimaryPagesExposeTheInstallExperience(t *testing.T) { //nolint:cyclop // The shared install contract checks each primary page explicitly.
	assetContracts.PrimaryPagesExposeTheInstallExperience(t)
}

func TestLibraryExposesDiscoverableCommandsAndInputParity(t *testing.T) {
	assetContracts.LibraryExposesDiscoverableCommandsAndInputParity(t)
}

func TestDesktopRailSeparatesUtilitiesFromAccount(t *testing.T) {
	assetContracts.DesktopRailSeparatesUtilitiesFromAccount(t)
}

func TestPagesUseSharedModernStyles(t *testing.T) {
	t.Parallel()

	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/setup", nil))
	styles := httptest.NewRecorder()
	handler.ServeHTTP(styles, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css", nil))
	home, settings := httptest.NewRecorder(), httptest.NewRecorder()
	open := server.New(server.Config{})
	open.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	open.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))

	if styles.Header().Get("Content-Type") != "text/css; charset=utf-8" {
		t.Fatalf("page = %q, styles = %d %q", page.Body.String(), styles.Code, styles.Body.String())
	}
	for body, fragments := range map[string][]string{
		page.Body.String():     {`class="brand-mark"`},
		styles.Body.String():   {"--signal:#c8f169", "--focus:#e4ff9c", `url("/static/cinema-backdrop.jpg")`, "flex-wrap:wrap", ".resume-link:focus-visible", ".resume-action", "margin-top:0;padding-top:0;border-top:0;background:none"},
		home.Body.String():     {`class="home-sections"`, `/static/app.css?v=electric-24`},
		settings.Body.String(): {"settings-page"},
	} {
		for _, fragment := range fragments {
			if !strings.Contains(body, fragment) {
				t.Fatalf("asset contract missing %q: %q", fragment, body)
			}
		}
	}
	if strings.Contains(home.Body.String(), "Your evening.") {
		t.Fatal("home still renders the removed greeting")
	}
	backdrop := httptest.NewRecorder()
	handler.ServeHTTP(backdrop, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/cinema-backdrop.jpg", nil))
	if backdrop.Code != http.StatusOK || backdrop.Header().Get("Content-Type") != "image/jpeg" || backdrop.Body.Len() < 100_000 {
		t.Fatalf("backdrop = %d %q, bytes = %d", backdrop.Code, backdrop.Header().Get("Content-Type"), backdrop.Body.Len())
	}
}

func TestMobileMediaHeroesStackTheirArtworkAndCopy(t *testing.T) {
	t.Parallel()

	styles := httptest.NewRecorder()
	server.New(server.Config{}).ServeHTTP(styles, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css", nil))
	css := styles.Body.String()
	for _, fragment := range []string{
		`body .media-hero{grid-template-columns:1fr;align-items:start;gap:1rem;padding:0 0 2rem}`,
		`body .media-hero>.hero-poster{width:min(100%,18rem);max-width:100%;min-width:0;height:auto;justify-self:start}`,
	} {
		if !strings.Contains(css, fragment) {
			t.Fatalf("mobile media hero stylesheet missing %q", fragment)
		}
	}
}

func TestCollectionPosterPlaceholderUsesOpenQuadrant(t *testing.T) {
	t.Parallel()

	response := httptest.NewRecorder()
	server.New(server.Config{}).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css", nil))
	css := response.Body.String()
	if !strings.Contains(css, ".curation-poster:before{") || !strings.Contains(css, ".curation-poster:has(> :nth-child(3)):not(:has(> :nth-child(4))):before{") || !strings.Contains(css, ".curation-poster:has(> :nth-child(4)):before{") {
		t.Fatalf("collection poster placeholder does not use the open quadrant")
	}
}

func TestViewerCanChooseDarkLightOrSystemTheme(t *testing.T) {
	assetContracts.ViewerCanChooseDarkLightOrSystemTheme(t)
}

func TestSettingsAndSharedPageFamiliesUseSignalLayout(t *testing.T) {
	assetContracts.SettingsAndSharedPageFamiliesUseSignalLayout(t)
}

func TestPausingSavesProgressWithoutMarkingMediaWatched(t *testing.T) {
	t.Parallel()

	response := httptest.NewRecorder()
	server.New(server.Config{}).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/player.js", nil))
	if !strings.Contains(response.Body.String(), `addEventListener("pause", () => save(false))`) {
		t.Fatalf("player script = %q", response.Body.String())
	}
}

func TestHomeIsInstallableAsAWebApp(t *testing.T) {
	t.Parallel()
	servertest.AssertInstallableWebApp(t, server.New(server.Config{}))
}

func TestBrandUsesFoldedPlayLogo(t *testing.T) {
	t.Parallel()
	servertest.AssertBrandLogo(t, server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true}), []string{`<img class="brand-mark" src="/static/icon.svg?v=8"`}, []string{`d="M160 112 384 256 160 304Z"`, `d="m160 336 128-28-128 92Z"`}, []string{`body .brand-icon{box-shadow:none;filter:none;border-radius:10px}`}, nil)
}

func TestPlayerHTMLStartsInDefaultDarkTheme(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{})
	for _, path := range []string{"/", "/settings", "/account", "/?lang=ar"} {
		page := httptest.NewRecorder()
		handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `data-theme="dark"`) {
			t.Errorf("%s did not render the default theme before JavaScript", path)
		}
	}
}
