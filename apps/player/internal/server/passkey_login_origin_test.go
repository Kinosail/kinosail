package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPasskeyLoginBeginRejectsWrongOriginBeforeRateLimit(t *testing.T) {
	t.Parallel()
	auth := newPasskeyAuth("https://media.example:38127", newProfileStore(t.TempDir(), nil))
	mux := http.NewServeMux()
	called := false
	auth.register(mux, func(string) bool {
		called = true
		return false
	})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://other.example:38127/api/v1/passkeys/login/begin", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusMisdirectedRequest || called {
		t.Fatalf("wrong-origin login begin = %d, rate limiter called = %t", response.Code, called)
	}
}
