package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestInvalidExplicitCredentialsDoNotFallBackToAValidCookie(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")

	for name, credential := range map[string]struct{ path, header, value string }{
		"Bearer":               {"/api/v1/settings", "Authorization", "Bearer invalid"},
		"ApiKey":               {"/api/v1/settings", "ApiKey", "invalid"},
		"X-Emby-Token":         {"/api/v1/settings", "X-Emby-Token", "invalid"},
		"X-MediaBrowser-Token": {"/api/v1/settings", "X-MediaBrowser-Token", "invalid"},
		"MediaBrowser":         {"/api/v1/settings", "Authorization", `MediaBrowser Token="invalid"`},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, credential.path, nil)
			request.AddCookie(owner)
			if credential.header != "" {
				request.Header.Set(credential.header, credential.value)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("invalid explicit credential fell back to cookie: %d %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestCredentialRateLimitSpansWebAndAPI(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	_ = signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	var last *httptest.ResponseRecorder
	for attempt := range 11 {
		var request *http.Request
		switch attempt % 2 {
		case 0:
			request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", strings.NewReader("name=Owner&password=wrong-password"))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		default:
			body, _ := json.Marshal(map[string]string{"name": "Owner", "password": "wrong-password"})
			request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/session", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
		}
		request.RemoteAddr = "192.0.2.80:1234"
		last = httptest.NewRecorder()
		handler.ServeHTTP(last, request)
		if attempt < 10 && last.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d %q", attempt+1, last.Code, last.Body.String())
		}
	}
	if last.Code != http.StatusTooManyRequests || last.Header().Get("Retry-After") != "60" {
		t.Fatalf("cross-adapter rate limit = %d headers=%v body=%q", last.Code, last.Header(), last.Body.String())
	}
}
