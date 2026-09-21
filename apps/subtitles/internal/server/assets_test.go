package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
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

func TestPrimaryPagesExposeTheInstallExperience(t *testing.T) {
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
	assertAssetContains(t, "setup", page.Body.String(), `class="brand-mark"`)
	assertAssetContains(t, "styles", styles.Body.String(), "--signal:#c8f169", "--focus:#e4ff9c", `url("/static/cinema-backdrop.jpg")`, "flex-wrap:wrap")
	assertAssetContains(t, "home", home.Body.String(), `class="library-masthead"`)
	assertAssetContains(t, "settings", settings.Body.String(), "settings-page")
	backdrop := httptest.NewRecorder()
	handler.ServeHTTP(backdrop, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/cinema-backdrop.jpg", nil))
	if backdrop.Code != http.StatusOK || backdrop.Header().Get("Content-Type") != "image/jpeg" || backdrop.Body.Len() < 100_000 {
		t.Fatalf("backdrop = %d %q, bytes = %d", backdrop.Code, backdrop.Header().Get("Content-Type"), backdrop.Body.Len())
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

func assertAssetContains(t *testing.T, name, body string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(body, fragment) {
			t.Fatalf("%s missing %q: %q", name, fragment, body)
		}
	}
}

func TestBrandUsesPairedCaptionsLogo(t *testing.T) {
	t.Parallel()
	servertest.AssertBrandLogo(t, server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true}), []string{`<img class="brand-mark" src="/static/icon.svg?v=11"`}, []string{`d="M128 144h256v96H184l-56 48Z"`, `d="M128 272h256v112l-56-32H128Z"`}, []string{`.brand-icon,.brand-mark{width:38px;height:38px;border-radius:11px}`}, []string{`d="m224 150 96 76-96 76z"`})
}
