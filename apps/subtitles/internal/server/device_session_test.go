package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestNamedDeviceSessionsAreHashedVisibleAndRevocable(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	handler := server.New(server.Config{DataDir: dataDir, RequireAuth: true})
	setup := apiCall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "totp": true})
	var enrollment struct {
		Token string
		TOTP  struct{ Secret string }
	}
	mustJSON(t, setup, &enrollment)
	assertAPIBody(t, apiCall(t, handler, enrollment.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": testTOTP(t, enrollment.TOTP.Secret, time.Now())}), http.StatusOK, `"enabled":true`)
	owner := &http.Cookie{Name: "__Host-kinosail_session", Value: enrollment.Token, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/session", strings.NewReader("name=Owner&password=owner-password&device=Living+Room+TV&code="+testTOTP(t, enrollment.TOTP.Secret, time.Now())))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var created struct {
		Token string `json:"token"`
	}
	if json.Unmarshal(response.Body.Bytes(), &created) != nil || created.Token == "" {
		t.Fatalf("API session = %d %q", response.Code, response.Body.String())
	}
	stored := storedState(t, dataDir, "sessions.json")
	if strings.Contains(string(stored), created.Token) {
		t.Fatalf("persisted sessions expose secret: data=%q", stored)
	}
	settings := requestWithCookie(t, handler, http.MethodGet, "/settings", "", owner)
	match := regexp.MustCompile(`name="id" value="([a-f0-9]+)">Revoke Living Room TV`).FindStringSubmatch(settings.Body.String())
	if len(match) != 2 {
		t.Fatalf("named device not visible: %q", settings.Body.String())
	}
	revoke := requestWithCookieRequest(t, http.MethodPost, "/settings/sessions/device", "id="+match[1], owner)
	serveRequest(handler, revoke)
	api := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil)
	api.Header.Set("Authorization", "Bearer "+created.Token)
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, api)
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("revoked API session = %d %q", denied.Code, denied.Body.String())
	}
}
