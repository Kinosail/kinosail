package server_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

func signInTestProfile(t *testing.T, handler http.Handler, path, body string) *http.Cookie {
	t.Helper()
	if path == "/setup" && !strings.Contains(body, "totp=") {
		body += "&totp=true"
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if len(response.Result().Cookies()) != 1 {
		t.Fatalf("sign in = %d, cookies = %v", response.Code, response.Result().Cookies())
	}
	cookie := response.Result().Cookies()[0]
	if path == "/setup" {
		confirmTestFactor(t, handler, cookie, response)
	} else if response.Header().Get("Location") == "/account?mfa=required" {
		confirmTestFactor(t, handler, cookie, requestWithCookie(t, handler, http.MethodPost, "/account/mfa/setup", "", cookie))
	}
	return cookie
}

func confirmTestFactor(t *testing.T, handler http.Handler, cookie *http.Cookie, enrollment *httptest.ResponseRecorder) {
	t.Helper()
	secret := regexp.MustCompile(`<code>([A-Z2-7]{32})</code>`).FindStringSubmatch(enrollment.Body.String())
	if len(secret) != 2 {
		t.Fatalf("setup factor = %d %q", enrollment.Code, enrollment.Body.String())
	}
	confirmed := requestWithCookie(t, handler, http.MethodPost, "/account/mfa/enable", "code="+testTOTP(t, secret[1], time.Now()), cookie)
	if confirmed.Code != http.StatusSeeOther {
		t.Fatalf("confirm setup factor = %d %q", confirmed.Code, confirmed.Body.String())
	}
}
