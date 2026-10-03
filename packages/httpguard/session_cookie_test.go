package httpguard

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

//nolint:cyclop,gocognit // Each precedence case verifies selection and both CSRF outcomes together.
func TestScopedBrowserCookieUsesOneSessionForCSRF(t *testing.T) {
	t.Parallel()
	const scoped = "__Host-kinosail_player_session"
	for _, test := range []struct {
		name                 string
		own, legacy, sibling *http.Cookie
		want                 string
	}{
		{name: "legacy migration", legacy: &http.Cookie{Name: "__Host-kinosail_session", Value: "legacy", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode}, want: "legacy"},
		{name: "scoped wins", own: &http.Cookie{Name: scoped, Value: "own", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode}, legacy: &http.Cookie{Name: "__Host-kinosail_session", Value: "legacy", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode}, want: "own"},
		{name: "empty scoped blocks fallback", own: &http.Cookie{Name: scoped, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode}, legacy: &http.Cookie{Name: "__Host-kinosail_session", Value: "legacy", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode}},
		{name: "sibling ignored", sibling: &http.Cookie{Name: "__Host-kinosail_subtitles_session", Value: "sibling", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode}},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings", nil)
			for _, cookie := range []*http.Cookie{test.own, test.legacy, test.sibling} {
				if cookie != nil {
					request.AddCookie(cookie)
				}
			}
			WithSessionCookieName(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				if SessionCookieName(r) != scoped {
					t.Fatal("missing trusted app namespace")
				}
				cookie := BrowserSessionCookie(r)
				got := ""
				if cookie != nil {
					got = cookie.Value
				}
				if got != test.want {
					t.Fatalf("selected cookie = %q, want %q", got, test.want)
				}
				wantCSRF := ""
				if test.want != "" {
					wantCSRF = CSRFToken(test.want)
				}
				if CSRFForRequest(r) != wantCSRF {
					t.Fatal("CSRF did not use the selected cookie")
				}
				if test.want != "" {
					r.Header.Set("X-Kinosail-CSRF", CSRFToken("sibling"))
					if ValidCSRF(r) {
						t.Fatal("sibling CSRF accepted")
					}
					r.Header.Set("X-Kinosail-CSRF", wantCSRF)
					if !ValidCSRF(r) {
						t.Fatal("own CSRF rejected")
					}
				}
			}), scoped).ServeHTTP(httptest.NewRecorder(), request)
		})
	}
}

func TestMalformedScopedCookieCannotReviveLegacySession(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.Header.Set("Cookie", `__Host-kinosail_player_session="unterminated; __Host-kinosail_session=legacy`)
	WithSessionCookieName(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		cookie := BrowserSessionCookie(r)
		if cookie == nil || cookie.Name != "__Host-kinosail_player_session" || cookie.Value != "" || CSRFForRequest(r) != "" {
			t.Fatal("malformed scoped cookie revived legacy")
		}
	}), "__Host-kinosail_player_session").ServeHTTP(httptest.NewRecorder(), request)
}
