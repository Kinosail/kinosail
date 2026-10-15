package federation

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSSOSecondFactorLimitsAttemptsAndConsumesChallenge(t *testing.T) {
	t.Parallel()
	values := []webProfile{{ID: "viewer", MFA: true}, {ID: "second", MFA: true}, {ID: "third", MFA: true}, {ID: "fourth", MFA: true}, {ID: "fifth", MFA: true}}
	effects := &webEffects{}
	login := NewOIDCHTTP(OIDCConfig{}, webProfilesWithPersist(&values, nil), webHooks(effects), func(http.ResponseWriter, *http.Request, string) error { return nil })
	mux := http.NewServeMux()
	login.Register(mux)

	challenge, _ := login.BeginMFA("viewer")
	for range maxMFAFailures {
		assertMFAAttempt(t, mux, "10.0.0.1:1", challenge, "wrong", http.StatusUnauthorized)
	}
	if _, found := login.flow.MFAProfile(challenge); found {
		t.Fatal("challenge survived repeated failures")
	}
	assertMFAAttempt(t, mux, "10.0.0.1:1", challenge, "valid", http.StatusUnauthorized)

	challenge, _ = login.BeginMFA("viewer")
	for range mfaAttemptsPerProfile - maxMFAFailures {
		assertMFAAttempt(t, mux, "10.0.0.1:1", challenge, "wrong", http.StatusUnauthorized)
	}
	challenge, _ = login.BeginMFA("viewer")
	response := assertMFAAttempt(t, mux, "10.0.0.1:1", challenge, "valid", http.StatusTooManyRequests)
	if response.Header().Get("Retry-After") != "60" || effects.signIns != 0 {
		t.Fatalf("profile limit response = %v sign-ins %d", response.Header(), effects.signIns)
	}
	if _, found := login.flow.MFAProfile(challenge); !found {
		t.Fatal("rate-limited attempt consumed the challenge")
	}

	challenge, _ = login.BeginMFA("second")
	assertMFAAttempt(t, mux, "10.0.0.2:1", challenge, "valid", http.StatusSeeOther)
	challenge, _ = login.BeginMFA("second")
	assertMFAAttempt(t, mux, "10.0.0.2:1", challenge, "valid", http.StatusSeeOther)
	if effects.signIns != 2 {
		t.Fatalf("successful sign-ins = %d", effects.signIns)
	}

	attempts := mfaAttemptsPerProfile + 2
	for _, profile := range []string{"second", "third", "fourth"} {
		for range mfaAttemptsPerProfile {
			challenge, _ = login.BeginMFA(profile)
			if attempts > mfaAttemptsPerAddress {
				assertMFAAttempt(t, mux, "10.0.0.1:1", challenge, "valid", http.StatusTooManyRequests)
			} else {
				assertMFAAttempt(t, mux, "10.0.0.1:1", challenge, "wrong", http.StatusUnauthorized)
			}
			attempts++
		}
	}
	challenge, _ = login.BeginMFA("fifth")
	assertMFAAttempt(t, mux, "10.0.0.3:1", challenge, "valid", http.StatusSeeOther)
}

func assertMFAAttempt(t *testing.T, handler http.Handler, remote, challenge, code string, want int) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login/mfa", strings.NewReader("challenge="+challenge+"&code="+code))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.RemoteAddr = remote
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != want {
		t.Fatalf("MFA attempt from %s with %q = %d, want %d", remote, code, response.Code, want)
	}
	return response
}
