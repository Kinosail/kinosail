package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

func TestTrustedHTTPSBodyBoundPrecedesCSRFParsing(t *testing.T) {
	for _, input := range []struct {
		name, path         string
		chunked, oversized bool
	}{
		{"settings", "/settings/trusted-https", false, false},
		{"settings oversized", "/settings/trusted-https", false, true},
		{"settings chunked", "/settings/trusted-https", true, false},
		{"settings chunked oversized", "/settings/trusted-https", true, true},
		{"onboarding", "/onboarding/trusted-https", false, false},
		{"onboarding oversized", "/onboarding/trusted-https", false, true},
		{"onboarding chunked", "/onboarding/trusted-https", true, false},
		{"onboarding chunked oversized", "/onboarding/trusted-https", true, true},
	} {
		t.Run(input.name, func(t *testing.T) {
			assertTrustedHTTPSBodyBound(t, input.path, input.chunked, input.oversized)
		})
	}
}

func assertTrustedHTTPSBodyBound(t *testing.T, path string, chunked, oversized bool) {
	t.Helper()
	secret := "token"
	if oversized {
		secret = strings.Repeat("x", 16<<10)
	}
	body := url.Values{"domain": {"house"}, "token": {secret}, "address": {"192.168.1.2"}, "termsAccepted": {"true"}, "_csrf": {csrfToken("session")}}.Encode()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://kinosail.test"+path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://kinosail.test")
	request.Header.Set("User-Agent", "Browser")
	request.AddCookie(&http.Cookie{Name: "__Host-kinosail_session", Value: "session", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	if chunked {
		request.ContentLength = -1
		request.TransferEncoding = []string{"chunked"}
	}
	saves := 0
	form := trustedhttps.SettingsForm{Save: func(trustedhttps.SettingsInput) error { saves++; return nil }, Error: localizedError, Redirect: "/settings"}
	response := httptest.NewRecorder()
	security(form).ServeHTTP(response, request)
	if oversized && (saves != 0 || response.Code < 400) {
		t.Fatalf("oversized form reached save: path=%s chunked=%v status=%d saves=%d", path, chunked, response.Code, saves)
	}
	if !oversized && (saves != 1 || response.Code != http.StatusSeeOther) {
		t.Fatalf("normal CSRF form rejected: path=%s chunked=%v status=%d saves=%d", path, chunked, response.Code, saves)
	}
}
