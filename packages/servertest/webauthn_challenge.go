package servertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// WebAuthnChallenge obtains real ceremony options and their session cookie.
func WebAuthnChallenge(t testing.TB, handler http.HandlerFunc, request *http.Request) (string, *http.Cookie) {
	t.Helper()
	response := httptest.NewRecorder()
	handler(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("ceremony begin = %d: %s", response.Code, response.Body.String())
	}
	var options struct{ PublicKey struct{ Challenge string } }
	if err := json.Unmarshal(response.Body.Bytes(), &options); err != nil {
		t.Fatal(err)
	}
	cookies := response.Result().Cookies()
	if options.PublicKey.Challenge == "" || len(cookies) != 1 {
		t.Fatalf("challenge response = %+v, cookies %d", options, len(cookies))
	}
	return options.PublicKey.Challenge, cookies[0]
}
