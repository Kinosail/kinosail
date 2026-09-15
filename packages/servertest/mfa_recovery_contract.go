package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// MFARecoveryFixture preserves the app's initial Jellyfin setup, restart, and login copy.
type MFARecoveryFixture struct {
	New, Restart func(*testing.T, string) http.Handler
	TOTP         func(*testing.T, string, time.Time) string
	StoredState  func(*testing.T, string, string) []byte
	LoginLabel   string
}

// AssertTOTPAndOneTimeRecoveryCodesGatePasswordSessions preserves the complete real-handler MFA lifecycle.
func AssertTOTPAndOneTimeRecoveryCodesGatePasswordSessions(t *testing.T, fixture MFARecoveryFixture) {
	t.Parallel()

	data := t.TempDir()
	handler := fixture.New(t, data)
	var session struct {
		Token string `json:"token"`
	}
	MustJSON(t, APICall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password"}), &session)
	setup := APICall(t, handler, session.Token, http.MethodPost, "/api/v1/me/mfa/setup", map[string]any{})
	var enrollment struct {
		Secret        string   `json:"secret"`
		RecoveryCodes []string `json:"recoveryCodes"`
	}
	MustJSON(t, setup, &enrollment)
	if len(enrollment.RecoveryCodes) != 10 || enrollment.Secret == "" {
		t.Fatalf("MFA setup = %d %q", setup.Code, setup.Body.String())
	}
	code := fixture.TOTP(t, enrollment.Secret, time.Now())
	confirmed := APICall(t, handler, session.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": code})
	AssertAPIBody(t, confirmed, http.StatusOK, `"enabled":true`)
	lastFactor := APICall(t, handler, session.Token, http.MethodDelete, "/api/v1/me/mfa", map[string]any{"code": fixture.TOTP(t, enrollment.Secret, time.Now())})
	AssertAPIBody(t, lastFactor, http.StatusConflict, "owner must retain a passkey or authenticator")
	missing := APICall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Owner", "password": "owner-password"})
	AssertAPIBody(t, missing, http.StatusUnauthorized, `"mfaRequired":true`)
	valid := APICall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Owner", "password": "owner-password", "code": fixture.TOTP(t, enrollment.Secret, time.Now())})
	if valid.Code != http.StatusCreated {
		t.Fatalf("TOTP login = %d %q", valid.Code, valid.Body.String())
	}
	recovery := enrollment.RecoveryCodes[0]
	first := APICall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Owner", "password": "owner-password", "code": recovery})
	if first.Code != http.StatusCreated {
		t.Fatalf("recovery login = %d %q", first.Code, first.Body.String())
	}
	second := APICall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Owner", "password": "owner-password", "code": recovery})
	AssertAPIBody(t, second, http.StatusUnauthorized, "invalid authentication code")

	handler = fixture.Restart(t, data)
	restarted := APICall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Owner", "password": "owner-password"})
	AssertAPIBody(t, restarted, http.StatusUnauthorized, `"mfaRequired":true`)
	web := APICall(t, handler, "", http.MethodGet, "/login", nil)
	AssertAPIBody(t, web, http.StatusOK, fixture.LoginLabel, `name="code"`)
	profiles := fixture.StoredState(t, data, "profiles.json")
	if strings.Contains(string(profiles), recovery) {
		t.Fatalf("recovery code stored in plaintext: %q", profiles)
	}
	jellyfin := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/Users/AuthenticateByName", strings.NewReader(`{"Username":"Owner","Pw":"owner-password"}`))
	jellyfin.Header.Set("Content-Type", "application/json")
	jellyfinResponse := httptest.NewRecorder()
	handler.ServeHTTP(jellyfinResponse, jellyfin)
	if jellyfinResponse.Code != http.StatusForbidden || !strings.Contains(jellyfinResponse.Body.String(), "Quick Connect") {
		t.Fatalf("Jellyfin MFA password login = %d %q", jellyfinResponse.Code, jellyfinResponse.Body.String())
	}
}
