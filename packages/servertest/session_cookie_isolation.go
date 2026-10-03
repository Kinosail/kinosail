package servertest

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

// AssertBrowserSessionCookieIsolation checks real app handlers with a shared host cookie jar.
//
//nolint:cyclop,gocognit,funlen // The complete real-handler browser journey preserves its cookie and security assertions together.
func AssertBrowserSessionCookieIsolation(t *testing.T, handler http.Handler, origin, ownName, siblingName string) {
	t.Helper()
	base, err := url.Parse(origin)
	if err != nil {
		t.Fatal(err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequestWithContext(t.Context(), method, origin+path, strings.NewReader(body))
		request.Header.Set("User-Agent", "Kinosail synthetic browser")
		request.Header.Set("Origin", origin)
		request.Header.Set("Sec-Fetch-Site", "same-origin")
		if body != "" {
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		for _, cookie := range jar.Cookies(base) {
			request.AddCookie(cookie)
			if cookie.Name == ownName {
				request.Header.Set("X-Kinosail-CSRF", httpguard.CSRFToken(cookie.Value))
			}
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		jar.SetCookies(base, response.Result().Cookies())
		return response
	}
	response := call(http.MethodPost, "/setup", "name=Owner&password=owner-password")
	if response.Code != http.StatusSeeOther {
		t.Fatalf("setup status = %d", response.Code)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != ownName {
		t.Fatalf("session cookie name = %v, want %s", cookieNames(cookies), ownName)
	}
	own := cookies[0]
	if !own.Secure || !own.HttpOnly || own.SameSite != http.SameSiteStrictMode || own.Path != "/" || own.Domain != "" || own.MaxAge <= 0 || own.Expires.IsZero() {
		t.Fatal("session cookie lost its security or persistence attributes")
	}
	assertScopedBrowserCSRF(t, handler, origin, own)
	sibling := &http.Cookie{Name: siblingName, Value: "sibling-fixture", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 86400}
	jar.SetCookies(base, []*http.Cookie{sibling})
	if response := call(http.MethodGet, "/account", ""); response.Code != http.StatusOK {
		t.Fatalf("sibling login lost own session: %d", response.Code)
	}
	if response := call(http.MethodPost, "/logout", ""); response.Code != http.StatusSeeOther {
		t.Fatalf("logout status = %d", response.Code)
	}
	found := false
	for _, cookie := range jar.Cookies(base) {
		if cookie.Name == ownName {
			t.Error("logout retained own cookie")
		}
		if cookie.Name == siblingName && cookie.Value == sibling.Value {
			found = true
		}
	}
	if !found {
		t.Fatal("logout removed sibling cookie")
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, origin+"/account", nil)
	request.AddCookie(own)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" {
		t.Fatalf("logged-out credential accepted: %d", response.Code)
	}
}

func cookieNames(cookies []*http.Cookie) []string {
	names := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		names = append(names, cookie.Name)
	}
	return names
}

func assertScopedBrowserCSRF(t *testing.T, handler http.Handler, origin string, own *http.Cookie) {
	t.Helper()
	for _, csrf := range []string{"", httpguard.CSRFToken("sibling-session")} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, origin+"/logout", nil)
		request.Header.Set("User-Agent", "Kinosail synthetic browser")
		request.Header.Set("Origin", origin)
		request.Header.Set("Sec-Fetch-Site", "same-origin")
		request.Header.Set("X-Kinosail-CSRF", csrf)
		request.AddCookie(own)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden || len(response.Result().Cookies()) != 0 {
			t.Fatalf("invalid scoped CSRF logout = %d", response.Code)
		}
	}
}

// AssertLegacyBrowserCookieMigration checks migration through the real access boundary.
//
//nolint:cyclop // One real-handler lifecycle checks migration attributes and invalid-cookie precedence.
func AssertLegacyBrowserCookieMigration(t *testing.T, handler http.Handler, origin, ownName string) {
	t.Helper()
	setup := httptest.NewRequestWithContext(t.Context(), http.MethodPost, origin+"/setup", strings.NewReader("name=Owner&password=owner-password"))
	setup.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, setup)
	if response.Code != http.StatusSeeOther || len(response.Result().Cookies()) != 1 {
		t.Fatalf("setup status = %d", response.Code)
	}
	original := *response.Result().Cookies()[0]
	legacy := original //nolint:gosec // Preserve the real secure response; only the pre-upgrade name changes.
	legacy.Name = httpguard.LegacySessionCookieName
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, origin+"/account", nil)
	request.AddCookie(&legacy)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	cookies := response.Result().Cookies()
	if response.Code != http.StatusOK || len(cookies) != 1 {
		t.Fatalf("legacy migration status = %d, cookies = %d", response.Code, len(cookies))
	}
	migrated := cookies[0]
	if migrated.Name != ownName || migrated.Value != legacy.Value || !migrated.Expires.Equal(original.Expires) || migrated.MaxAge > original.MaxAge {
		t.Fatal("migration changed the token or extended its expiry")
	}
	// A revoked app cookie must not fall through to a still-valid legacy credential.
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, origin+"/account", nil)
	request.AddCookie(&legacy)
	invalid := original //nolint:gosec // This negative fixture retains the secure cookie attributes.
	invalid.Value = "revoked-app-session"
	request.AddCookie(&invalid)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" || len(response.Result().Cookies()) != 0 {
		t.Fatal("invalid app cookie revived legacy authentication")
	}
}
