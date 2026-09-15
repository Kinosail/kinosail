package servertest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// LanguagePreferenceIsAvailableThroughAPI preserves the original locale preference regression.
func (fixture LocaleWebFixture) LanguagePreferenceIsAvailableThroughAPI(t *testing.T) {
	t.Parallel()
	handler, token := fixture.Server(t)
	response := APICall(t, handler, token, http.MethodPut, "/api/v1/me/language", map[string]any{"language": "es"})
	AssertAPIBody(t, response, http.StatusOK, `"language":"es"`)
	if len(response.Result().Cookies()) != 1 || response.Result().Cookies()[0].Value != "es" {
		t.Fatalf("language cookie = %v", response.Result().Cookies())
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.AddCookie(response.Result().Cookies()[0])
	me := httptest.NewRecorder()
	handler.ServeHTTP(me, request)
	AssertAPIBody(t, me, http.StatusOK, `"language":"es"`)
	AssertAPIBody(t, me, http.StatusOK, `"tag":"ar","name":"العربية","direction":"rtl"`)
	AssertAPIBody(t, me, http.StatusOK, `"tag":"hi-IN"`)
	AssertAPIBody(t, me, http.StatusOK, `"tag":"ckb"`)
	AssertAPIBody(t, APICall(t, handler, token, http.MethodPut, "/api/v1/me/language", map[string]any{"language": "xx"}), http.StatusBadRequest, `"error"`)
}

// BrowserLanguageIsMatchedAndCanReturnToAutomatic preserves the original locale preference regression.
func (fixture LocaleWebFixture) BrowserLanguageIsMatchedAndCanReturnToAutomatic(t *testing.T) {
	t.Parallel()
	handler, token := fixture.Server(t)

	automatic := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/me", nil)
	automatic.Header.Set("Authorization", "Bearer "+token)
	automatic.Header.Set("Accept-Language", "fr-CA,es;q=0.8,en;q=0.5")
	matched := httptest.NewRecorder()
	handler.ServeHTTP(matched, automatic)
	AssertAPIBody(t, matched, http.StatusOK, `"language":"fr"`)
	AssertAPIBody(t, matched, http.StatusOK, `"languagePreference":"auto"`)

	selected := APICall(t, handler, token, http.MethodPut, "/api/v1/me/language", map[string]any{"language": "de-DE"})
	AssertAPIBody(t, selected, http.StatusOK, `"language":"de"`)
	if len(selected.Result().Cookies()) != 1 || selected.Result().Cookies()[0].Value != "de" {
		t.Fatalf("selected language cookie = %v", selected.Result().Cookies())
	}

	reset := APICall(t, handler, token, http.MethodPut, "/api/v1/me/language", map[string]any{"language": "auto"})
	AssertAPIBody(t, reset, http.StatusOK, `"language":"en"`)
	AssertAPIBody(t, reset, http.StatusOK, `"languagePreference":"auto"`)
	if len(reset.Result().Cookies()) != 1 || reset.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatalf("reset language cookie = %v", reset.Result().Cookies())
	}
}
