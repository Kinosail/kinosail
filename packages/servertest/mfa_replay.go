package servertest

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func AssertTOTPCodesCannotBeReplayed(t *testing.T, handler http.Handler) {
	t.Helper()
	var owner struct {
		Token string
		TOTP  struct{ Secret string }
	}
	var viewer struct{ Token string }
	MustJSON(t, APICall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "totp": true}), &owner)
	AssertAPIBody(t, APICall(t, handler, owner.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": TestTOTP(t, owner.TOTP.Secret, time.Now())}), http.StatusOK, `"enabled":true`)
	if created := APICall(t, handler, owner.Token, http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Viewer", "password": "viewer-password"}); created.Code != http.StatusCreated {
		t.Fatalf("create Viewer = %d %q", created.Code, created.Body.String())
	}
	MustJSON(t, APICall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Viewer", "password": "viewer-password"}), &viewer)
	var enrollment struct{ Secret string }
	MustJSON(t, APICall(t, handler, viewer.Token, http.MethodPost, "/api/v1/me/mfa/setup", map[string]any{}), &enrollment)
	now := stableTOTPWindow()
	AssertAPIBody(t, APICall(t, handler, viewer.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": TestTOTP(t, enrollment.Secret, now)}), http.StatusOK, `"enabled":true`)

	previous, current, next := TestTOTP(t, enrollment.Secret, now.Add(-30*time.Second)), TestTOTP(t, enrollment.Secret, now), TestTOTP(t, enrollment.Secret, now.Add(30*time.Second))
	login := func(code string) *httptest.ResponseRecorder {
		return APICall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Viewer", "password": "viewer-password", "code": code})
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
	if disabled := APICall(t, handler, session.Token, http.MethodDelete, "/api/v1/me/mfa", map[string]any{"code": next}); disabled.Code != http.StatusNoContent {
		t.Fatalf("MFA disable with a fresh code = %d %q", disabled.Code, disabled.Body.String())
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
