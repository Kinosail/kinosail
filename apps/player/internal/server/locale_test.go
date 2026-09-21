package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestWebSurfacesCanUseSpanish(t *testing.T) {
	localeWebFixture.WebSurfacesCanUseSpanish(t, signInTestProfile, requestWithCookie, "Almacenado aquí. Transmitido directamente.", "Protección de inicio de sesión")
}

func TestLocalizationDoesNotTranslateLibraryContent(t *testing.T) {
	servertest.AssertLocalizationDoesNotTranslateLibraryContent(t, libraryAPIFixture)
}

func TestLocalizedPlayerUsesCatalogedPlaybackMode(t *testing.T) {
	servertest.AssertLocalizedPlayerUsesCatalogedPlaybackMode(t, libraryAPIFixture)
}

func TestSetupAndLoginCanUseSpanish(t *testing.T) { localeWebFixture.SetupAndLoginCanUseSpanish(t) }

func TestArabicLoginLocalizesTheRecoveryCodeLabel(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir()})
	login := httptest.NewRecorder()
	handler.ServeHTTP(login, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login?lang=ar", nil))
	if !strings.Contains(login.Body.String(), `lang="ar" dir="rtl"`) || !strings.Contains(login.Body.String(), "رمز المصادقة أو الاسترداد") || strings.Contains(login.Body.String(), "Authentication or recovery code") {
		t.Fatalf("Arabic recovery code label = %q", login.Body.String())
	}
}

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
