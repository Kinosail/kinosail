package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestTOTPAndOneTimeRecoveryCodesGatePasswordSessions(t *testing.T) {
	servertest.AssertTOTPAndOneTimeRecoveryCodesGatePasswordSessions(t, servertest.MFARecoveryFixture{New: newMFARecoveryServer, Restart: restartMFARecoveryServer, TOTP: testTOTP, StoredState: storedState, LoginLabel: "Authentication or recovery code"})
}

func TestRecoveryCodesWorkThroughWebLoginAndAccountDisable(t *testing.T) {
	handler := newJellyfinServer(t, server.Config{DataDir: t.TempDir(), RequireAuth: true})
	var owner struct {
		Token string
		TOTP  struct{ Secret string }
	}
	mustJSON(t, apiCall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "totp": true}), &owner)
	assertAPIBody(t, apiCall(t, handler, owner.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": testTOTP(t, owner.TOTP.Secret, time.Now())}), http.StatusOK, `"enabled":true`)
	created := apiCall(t, handler, owner.Token, http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Viewer", "password": "viewer-password"})
	if created.Code != http.StatusCreated {
		t.Fatalf("create Viewer = %d %q", created.Code, created.Body.String())
	}
	var viewer struct{ Token string }
	mustJSON(t, apiCall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Viewer", "password": "viewer-password"}), &viewer)
	setup := apiCall(t, handler, viewer.Token, http.MethodPost, "/api/v1/me/mfa/setup", map[string]any{})
	var enrollment struct {
		Secret        string
		RecoveryCodes []string
	}
	mustJSON(t, setup, &enrollment)
	assertAPIBody(t, apiCall(t, handler, viewer.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": testTOTP(t, enrollment.Secret, time.Now())}), http.StatusOK, `"enabled":true`)

	login := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", strings.NewReader("name=Viewer&password=viewer-password&code="+enrollment.RecoveryCodes[0]))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loggedIn := httptest.NewRecorder()
	handler.ServeHTTP(loggedIn, login)
	if loggedIn.Code != http.StatusSeeOther || len(loggedIn.Result().Cookies()) != 1 {
		t.Fatalf("recovery web login = %d cookies=%v", loggedIn.Code, loggedIn.Result().Cookies())
	}
	disabled := requestWithCookie(t, handler, http.MethodPost, "/account/mfa/disable", "code="+enrollment.RecoveryCodes[1], loggedIn.Result().Cookies()[0])
	if disabled.Code != http.StatusSeeOther || disabled.Header().Get("Location") != "/account" {
		t.Fatalf("recovery web disable = %d location=%q", disabled.Code, disabled.Header().Get("Location"))
	}
	withoutCode := apiCall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Viewer", "password": "viewer-password"})
	if withoutCode.Code != http.StatusCreated {
		t.Fatalf("login after recovery disable = %d %q", withoutCode.Code, withoutCode.Body.String())
	}
}

func TestDefaultMFADoesNotBlockOpenInitialServer(t *testing.T) {
	handler := server.New(server.Config{})
	response := apiCall(t, handler, "", http.MethodGet, "/api/v1/library", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("initial open library = %d %q", response.Code, response.Body.String())
	}
}

var enrollTestAPIFactor = servertest.EnrollTestAPIFactor

func disableTestMFA(t *testing.T, handler http.Handler, token string) {
	t.Helper()
	response := apiCall(t, handler, token, http.MethodPut, "/api/v1/settings/mfa", map[string]any{"required": false})
	if response.Code != http.StatusOK {
		t.Fatalf("disable test MFA = %d %q", response.Code, response.Body.String())
	}
}

var testTOTP = servertest.TestTOTP
