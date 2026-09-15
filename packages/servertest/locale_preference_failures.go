package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// WebFailuresUseTheSelectedLanguage preserves the original localized error responses.
func (fixture LocaleWebFixture) WebFailuresUseTheSelectedLanguage(t *testing.T, signIn func(*testing.T, http.Handler, string, string) *http.Cookie, web AuthCookieRequest) {
	t.Parallel()
	handler := fixture.NewHandler(t, true)
	owner := signIn(t, handler, "/setup", "name=Owner&password=owner-password")
	login := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login?lang=es", strings.NewReader("name=Owner&password=wrong-password"))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, login)
	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), "credenciales no válidas") {
		t.Fatalf("Spanish login failure = %d %q", response.Code, response.Body.String())
	}
	invalidSetting := web(t, handler, http.MethodPost, "/settings/server?lang=es", "name=", owner)
	assertSpanishFailure(t, invalidSetting, http.StatusBadRequest)
	invalidPlaylist := web(t, handler, http.MethodPost, "/playlists?lang=es", "name=", owner)
	assertSpanishFailure(t, invalidPlaylist, http.StatusBadRequest)
	for path, message := range map[string]string{"/progress/missing?lang=es": "progreso no válido", "/watch-together?lang=es": "El estado Watch Together no es válido"} {
		assertSpanishFailureMessage(t, web(t, handler, http.MethodPost, path, "seconds=0", owner), http.StatusBadRequest, message)
	}
	sso := httptest.NewRecorder()
	handler.ServeHTTP(sso, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login/oidc?lang=es", nil))
	assertSpanishFailureMessage(t, sso, http.StatusServiceUnavailable, "SSO no está disponible")
}

func assertSpanishFailure(t *testing.T, response *httptest.ResponseRecorder, status int) {
	t.Helper()
	assertSpanishFailureMessage(t, response, status, "no se pudo completar la solicitud")
}

func assertSpanishFailureMessage(t *testing.T, response *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	if response.Code != status || !strings.Contains(response.Body.String(), `lang="es"`) || !strings.Contains(response.Body.String(), message) {
		t.Fatalf("Spanish failure = %d %q", response.Code, response.Body.String())
	}
}
