package servertest

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// WebLanguageSelectionUsesTheSharedPreferenceOperation preserves the original locale preference regression.
func (fixture LocaleWebFixture) WebLanguageSelectionUsesTheSharedPreferenceOperation(t *testing.T) {
	t.Parallel()
	handler := fixture.NewHandler(t, false)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/language", strings.NewReader("language=pt-PT"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Referer", "http://example.com/settings?section=appearance")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/settings?section=appearance" || len(response.Result().Cookies()) != 1 || response.Result().Cookies()[0].Value != "pt-PT" {
		t.Fatalf("web language selection = %d %q %v", response.Code, response.Header().Get("Location"), response.Result().Cookies())
	}
	login := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login", nil)
	login.AddCookie(response.Result().Cookies()[0])
	selected := httptest.NewRecorder()
	handler.ServeHTTP(selected, login)
	if !strings.Contains(selected.Body.String(), `<option value="pt-PT" selected>`) || strings.Contains(selected.Body.String(), `<option value="auto" selected>`) {
		t.Fatalf("saved picker preference = %q", selected.Body.String())
	}

	unsafe := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/language", strings.NewReader("language=de"))
	unsafe.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	unsafe.Header.Set("Referer", "https://attacker.example/steal")
	blocked := httptest.NewRecorder()
	handler.ServeHTTP(blocked, unsafe)
	if blocked.Code != http.StatusSeeOther || blocked.Header().Get("Location") != "/" {
		t.Fatalf("external return path = %d %q", blocked.Code, blocked.Header().Get("Location"))
	}
}

// AuthenticatedLanguagePickerCarriesCSRFToken preserves the original locale preference regression.
func (fixture LocaleWebFixture) AuthenticatedLanguagePickerCarriesCSRFToken(t *testing.T, signIn func(*testing.T, http.Handler, string, string) *http.Cookie, web AuthCookieRequest) {
	t.Parallel()
	handler := fixture.NewHandler(t, true)
	cookie := signIn(t, handler, "/setup", "name=Owner&password=owner-password")
	tokenPattern := regexp.MustCompile(`<form class="language-picker"[^>]*><input type="hidden" name="_csrf" value="([A-Za-z0-9_-]+)">`)
	var token string
	for _, path := range []string{"/", "/settings"} {
		page := web(t, handler, http.MethodGet, path, "", cookie)
		if page.Code != http.StatusOK {
			t.Fatalf("%s page = %d %q", path, page.Code, page.Body.String())
		}
		match := tokenPattern.FindStringSubmatch(page.Body.String())
		if len(match) != 2 {
			t.Fatalf("language picker CSRF token missing on %s: %q", path, page.Body.String())
		}
		token = match[1]
	}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/language", strings.NewReader("language=es&_csrf="+token))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://example.com")
	request.Header.Set("User-Agent", "Mozilla/5.0")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" {
		t.Fatalf("authenticated language selection = %d %q", response.Code, response.Body.String())
	}
}
