package httpguard

import (
	"context"
	"net/http"
	"strings"
)

const LegacySessionCookieName = "__Host-kinosail_session"

type sessionCookieNameKey struct{}

// WithSessionCookieName supplies an app-owned name before authentication and CSRF.
// Names come from the composition root, never request headers or URL values.
func WithSessionCookieName(next http.Handler, name string) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		next.ServeHTTP(writer, request.WithContext(context.WithValue(request.Context(), sessionCookieNameKey{}, name)))
	})
}

// SessionCookieName returns the app's host-only browser session cookie name.
func SessionCookieName(request *http.Request) string {
	if request != nil {
		if name, _ := request.Context().Value(sessionCookieNameKey{}).(string); name != "" {
			return name
		}
	}
	return LegacySessionCookieName
}

// BrowserSessionCookie selects the app cookie before the pre-isolation cookie.
// Presence wins even for an empty or invalid value; fallback cannot revive it.
func BrowserSessionCookie(request *http.Request) *http.Cookie {
	if request == nil {
		return nil
	}
	name := SessionCookieName(request)
	if cookie, _ := request.Cookie(name); cookie != nil {
		return cookie
	}
	if name != LegacySessionCookieName {
		if hasCookieName(request, name) {
			return &http.Cookie{Name: name, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode}
		}
		cookie, _ := request.Cookie(LegacySessionCookieName)
		return cookie
	}
	return nil
}

// Preserve scoped presence when net/http rejects a malformed value.
func hasCookieName(request *http.Request, name string) bool {
	for _, header := range request.Header.Values("Cookie") {
		for _, part := range strings.Split(header, ";") {
			key, _, _ := strings.Cut(strings.TrimSpace(part), "=")
			if strings.TrimSpace(key) == name {
				return true
			}
		}
	}
	return false
}
