package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// LocaleWebFixture binds shared locale behavior to a real app handler.
type LocaleWebFixture struct {
	NewHandler func(*testing.T, bool) http.Handler
	Server     func(*testing.T) (http.Handler, string)
}

// InvalidLanguageSelectionHasNoSideEffects checks API and web rejection without cookie mutation.
func (fixture LocaleWebFixture) InvalidLanguageSelectionHasNoSideEffects(t *testing.T) {
	t.Helper()
	t.Parallel()
	handler, token := fixture.Server(t)
	selected := APICall(t, handler, token, http.MethodPut, "/api/v1/me/language", map[string]any{"language": "es"})
	if len(selected.Result().Cookies()) != 1 {
		t.Fatalf("selected language cookie = %v", selected.Result().Cookies())
	}
	invalid := APICall(t, handler, token, http.MethodPut, "/api/v1/me/language", map[string]any{"language": "xx"})
	AssertAPIBody(t, invalid, http.StatusBadRequest, `"error"`)
	if len(invalid.Result().Cookies()) != 0 {
		t.Fatalf("invalid language changed cookies = %v", invalid.Result().Cookies())
	}
	me := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/me", nil)
	me.Header.Set("Authorization", "Bearer "+token)
	me.AddCookie(selected.Result().Cookies()[0])
	unchanged := httptest.NewRecorder()
	handler.ServeHTTP(unchanged, me)
	AssertAPIBody(t, unchanged, http.StatusOK, `"language":"es"`)

	web := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/language", strings.NewReader("language=xx"))
	web.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	webResponse := httptest.NewRecorder()
	handler.ServeHTTP(webResponse, web)
	if webResponse.Code != http.StatusBadRequest || len(webResponse.Result().Cookies()) != 0 {
		t.Fatalf("invalid web language = %d cookies=%v", webResponse.Code, webResponse.Result().Cookies())
	}
}

// ExpandedLanguageTagsPreserveRegionalAndRTLSemantics checks regional and right-to-left rendering.
func (fixture LocaleWebFixture) ExpandedLanguageTagsPreserveRegionalAndRTLSemantics(t *testing.T) {
	t.Helper()
	t.Parallel()
	handler := fixture.NewHandler(t, true)
	for _, test := range []struct {
		tag, direction string
	}{
		{"hi-IN", "ltr"},
		{"zh-CN", "ltr"},
		{"ckb", "rtl"},
		{"ur-PK", "rtl"},
	} {
		t.Run(test.tag, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login", nil)
			request.Header.Set("Accept-Language", test.tag)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			wantHTML := `<html lang="` + test.tag + `" dir="` + test.direction + `"`
			if response.Code != http.StatusOK || response.Header().Get("Content-Language") != test.tag || !strings.Contains(response.Body.String(), wantHTML) || !strings.Contains(response.Body.String(), `value="`+test.tag+`"`) {
				t.Fatalf("%s login = %d %q", test.tag, response.Code, response.Body.String())
			}
		})
	}
}

// LanguagePickerOffersEveryLocaleOnTheHomePage checks full and compact picker parity.
func (fixture LocaleWebFixture) LanguagePickerOffersEveryLocaleOnTheHomePage(t *testing.T) {
	t.Helper()
	t.Parallel()
	handler := fixture.NewHandler(t, false)
	languagePage := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/language", nil)
	languagePage.Header.Set("Accept-Language", "hi-IN")
	full := httptest.NewRecorder()
	handler.ServeHTTP(full, languagePage)
	if full.Code != http.StatusOK || full.Header().Get("Content-Language") != "hi-IN" || !strings.Contains(full.Body.String(), `<option value="hi-IN">`) || !strings.Contains(full.Body.String(), `<option value="ckb">`) {
		t.Fatalf("language page = %d %q", full.Code, full.Body.String())
	}
	wantOptions := strings.Count(full.Body.String(), `<option value="`)
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	homeBody := home.Body.String()
	if home.Code != http.StatusOK || !strings.Contains(homeBody, `href="/language"`) || strings.Count(homeBody, `<option value="`) != wantOptions {
		t.Fatalf("home language picker = %d %q", home.Code, home.Body.String())
	}
}
