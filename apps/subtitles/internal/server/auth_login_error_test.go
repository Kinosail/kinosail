package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicPasswordLoginExplainsPasskeyRecoveryWithoutCreatingSession(t *testing.T) {
	t.Parallel()
	auth := &authentication{}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://subtitles.example/login", strings.NewReader("name=owner&password=not-a-real-password"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	Remote(http.HandlerFunc(auth.login)).ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || !strings.HasPrefix(response.Header().Get("Content-Type"), "text/html") || !strings.Contains(response.Body.String(), "Use a passkey to sign in.") || !strings.Contains(response.Body.String(), "Return to Subtitles") {
		t.Fatalf("public login rejection = %d %q", response.Code, response.Body.String())
	}
	if len(response.Header().Values("Set-Cookie")) != 0 || strings.Contains(response.Body.String(), "not-a-real-password") {
		t.Fatal("rejection created a session or exposed submitted credentials")
	}
}
