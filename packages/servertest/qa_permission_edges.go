package servertest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// AssertPasskeyBeginRoutesFailClosedWhenConfigurationIsInvalid preserves the Player regression against the supplied app bindings.
func AssertPasskeyBeginRoutesFailClosedWhenConfigurationIsInvalid(t *testing.T, register, login http.HandlerFunc) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/passkeys/register/begin", nil)
	response := httptest.NewRecorder()
	register(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("invalid passkey registration = %d %q", response.Code, response.Body.String())
	}

	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/passkeys/login/begin", nil)
	response = httptest.NewRecorder()
	login(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("invalid passkey login = %d %q", response.Code, response.Body.String())
	}
}
