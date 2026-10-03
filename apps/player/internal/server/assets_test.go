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

func TestPrimaryPagesExposeTheInstallExperience(t *testing.T) { //nolint:cyclop // The shared install contract checks each primary page explicitly.
	assetContracts.PrimaryPagesExposeTheInstallExperience(t)
}

func TestLibraryExposesDiscoverableCommandsAndInputParity(t *testing.T) {
	assetContracts.LibraryExposesDiscoverableCommandsAndInputParity(t)
}

func TestViewerCanChooseDarkLightOrSystemTheme(t *testing.T) {
	assetContracts.ViewerCanChooseDarkLightOrSystemTheme(t)
}

func TestSettingsAndSharedPageFamiliesUseSignalLayout(t *testing.T) {
	assetContracts.SettingsAndSharedPageFamiliesUseSignalLayout(t)
}

func TestHomeIsInstallableAsAWebApp(t *testing.T) {
	t.Parallel()
	servertest.AssertInstallableWebApp(t, server.New(server.Config{}))
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
