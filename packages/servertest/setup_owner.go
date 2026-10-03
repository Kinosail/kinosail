package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// SetupOwnerCookie creates the shared local Owner fixture and returns its secure session.
func SetupOwnerCookie(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/setup", strings.NewReader("name=Owner&password=owner-password"))
	request.Host = "localhost:8080"
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(response, request)
	for _, cookie := range response.Result().Cookies() {
		if IsBrowserSessionCookie(cookie) {
			return cookie
		}
	}
	t.Fatalf("setup = %d %q", response.Code, response.Body.String())
	return nil
}

// IsBrowserSessionCookie recognizes each app's session response and legacy fixtures.
func IsBrowserSessionCookie(cookie *http.Cookie) bool {
	if cookie == nil {
		return false
	}
	switch cookie.Name {
	case "__Host-kinosail_session", "__Host-kinosail_player_session", "__Host-kinosail_subtitles_session":
		return true
	default:
		return false
	}
}
