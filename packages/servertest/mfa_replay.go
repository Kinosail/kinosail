package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func AssertTOTPCodesCannotBeReplayed(t *testing.T, handler http.Handler) {
	t.Helper()
	var owner struct {
		Token string
		TOTP  struct{ Secret string }
	}
	MustJSON(t, APICall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "totp": true}), &owner)
	now := stableTOTPWindow()
	previous, current, next := TestTOTP(t, owner.TOTP.Secret, now.Add(-30*time.Second)), TestTOTP(t, owner.TOTP.Secret, now), TestTOTP(t, owner.TOTP.Secret, now.Add(30*time.Second))
	AssertAPIBody(t, APICall(t, handler, owner.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": current}), http.StatusOK, `"enabled":true`)
	login := func(code string) *httptest.ResponseRecorder {
		return APICall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Owner", "password": "owner-password", "code": code})
	}
	if first := login(previous); first.Code != http.StatusCreated {
		t.Fatalf("first login with a fresh code = %d %q", first.Code, first.Body.String())
	}
	if replay := login(previous); replay.Code != http.StatusUnauthorized {
		t.Fatalf("replayed login code = %d %q", replay.Code, replay.Body.String())
	}
	response := login(current)
	if response.Code != http.StatusCreated {
		t.Fatalf("login with the next window = %d %q", response.Code, response.Body.String())
	}
	var session struct{ Token string }
	MustJSON(t, response, &session)
	if disabled := APICall(t, handler, session.Token, http.MethodDelete, "/api/v1/me/mfa", map[string]any{"code": current}); disabled.Code != http.StatusUnauthorized {
		t.Fatalf("MFA disable with a replayed code = %d %q", disabled.Code, disabled.Body.String())
	}
	if disabled := APICall(t, handler, session.Token, http.MethodDelete, "/api/v1/me/mfa", map[string]any{"code": next}); disabled.Code != http.StatusConflict || !strings.Contains(disabled.Body.String(), "owner must retain") {
		t.Fatalf("MFA disable with a fresh code did not reach the Owner policy = %d %q", disabled.Code, disabled.Body.String())
	}
}

func stableTOTPWindow() time.Time {
	now := time.Now()
	if now.Unix()%30 >= 25 {
		time.Sleep(time.Until(time.Unix((now.Unix()/30+1)*30, 0)))
		now = time.Now()
	}
	return now
}
