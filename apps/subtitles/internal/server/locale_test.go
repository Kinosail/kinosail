package server_test

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestWebSurfacesCanUseSpanish(t *testing.T) {
	localeWebFixture.WebSurfacesCanUseSpanish(t, signInTestProfile, requestWithCookie, "SubDL")
}

func TestLocalizationDoesNotTranslateLibraryContent(t *testing.T) {
	servertest.AssertLocalizationDoesNotTranslateLibraryContent(t, libraryAPIFixture)
}

func TestLocalizedPlayerUsesCatalogedPlaybackMode(t *testing.T) {
	servertest.AssertLocalizedPlayerUsesCatalogedPlaybackMode(t, libraryAPIFixture)
}

func TestSetupAndLoginCanUseSpanish(t *testing.T) { localeWebFixture.SetupAndLoginCanUseSpanish(t) }

func TestWebFailuresUseTheSelectedLanguage(t *testing.T) {
	localeWebFixture.WebFailuresUseTheSelectedLanguage(t, signInTestProfile, requestWithCookie)
}

func TestLanguagePreferenceIsAvailableThroughAPI(t *testing.T) {
	localeWebFixture.LanguagePreferenceIsAvailableThroughAPI(t)
}

func TestBrowserLanguageIsMatchedAndCanReturnToAutomatic(t *testing.T) {
	localeWebFixture.BrowserLanguageIsMatchedAndCanReturnToAutomatic(t)
}

func TestLanguageSelectorIsAvailableBeforeSignInAndSupportsRTL(t *testing.T) {
	localeWebFixture.LanguageSelectorIsAvailableBeforeSignInAndSupportsRTL(t)
}

func TestSupportedLanguagesRenderTheLoginSurface(t *testing.T) {
	localeWebFixture.SupportedLanguagesRenderTheLoginSurface(t)
}

func TestWebLanguageSelectionUsesTheSharedPreferenceOperation(t *testing.T) {
	localeWebFixture.WebLanguageSelectionUsesTheSharedPreferenceOperation(t)
}

func TestAuthenticatedLanguagePickerCarriesCSRFToken(t *testing.T) {
	localeWebFixture.AuthenticatedLanguagePickerCarriesCSRFToken(t, signInTestProfile, requestWithCookie)
}
