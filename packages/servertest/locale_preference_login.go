package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// LanguageSelectorIsAvailableBeforeSignInAndSupportsRTL preserves the original locale preference regression.
func (fixture LocaleWebFixture) LanguageSelectorIsAvailableBeforeSignInAndSupportsRTL(t *testing.T) {
	t.Parallel()
	handler := fixture.NewHandler(t, true)
	login := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login", nil)
	login.Header.Set("Accept-Language", "de-DE,de;q=0.9")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, login)
	if response.Code != http.StatusOK || response.Header().Get("Content-Language") != "de" || !strings.Contains(response.Body.String(), `<html lang="de" dir="ltr"`) || !strings.Contains(response.Body.String(), `action="/language" method="post" aria-label="Sprache"`) || !strings.Contains(response.Body.String(), `<option value="auto" selected>`) || !strings.Contains(response.Body.String(), "Anmelden") {
		t.Fatalf("German login = %d %q %q", response.Code, response.Header().Get("Content-Language"), response.Body.String())
	}

	arabic := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login", nil)
	arabic.Header.Set("Accept-Language", "ar")
	rtl := httptest.NewRecorder()
	handler.ServeHTTP(rtl, arabic)
	if !strings.Contains(rtl.Body.String(), `<html lang="ar" dir="rtl"`) || !strings.Contains(rtl.Body.String(), "تسجيل الدخول") {
		t.Fatalf("Arabic login = %q", rtl.Body.String())
	}
}

// SupportedLanguagesRenderTheLoginSurface preserves the original locale preference regression.
func (fixture LocaleWebFixture) SupportedLanguagesRenderTheLoginSurface(t *testing.T) {
	t.Parallel()
	handler := fixture.NewHandler(t, true)
	translations := map[string]string{
		"en": "Sign in", "es": "Iniciar sesión", "de": "Anmelden", "fr": "Se connecter", "pt-BR": "Fazer login", "zh-Hans": "登录",
		"it": "Accedi", "nl": "Inloggen", "pl": "Zaloguj się", "ru": "Войти", "ja": "ログイン", "ko": "로그인", "ar": "تسجيل الدخول",
		"tr": "Oturum açın", "uk": "Увійти", "pt-PT": "Fazer login", "zh-Hant": "登入", "sv": "Logga in",
	}
	for tag, translated := range translations {
		t.Run(tag, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login", nil)
			request.Header.Set("Accept-Language", tag)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			direction := "ltr"
			if tag == "ar" {
				direction = "rtl"
			}
			if response.Code != http.StatusOK || response.Header().Get("Content-Language") != tag || !strings.Contains(response.Body.String(), `<html lang="`+tag+`" dir="`+direction+`"`) || !strings.Contains(response.Body.String(), translated) || !strings.Contains(response.Body.String(), `value="`+tag+`"`) {
				t.Fatalf("%s login = %d %q %q", tag, response.Code, response.Header().Get("Content-Language"), response.Body.String())
			}
		})
	}
}
