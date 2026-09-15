package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestTOTPAndOneTimeRecoveryCodesGatePasswordSessions(t *testing.T) {
	servertest.AssertTOTPAndOneTimeRecoveryCodesGatePasswordSessions(t, servertest.MFARecoveryFixture{New: newMFARecoveryServer, Restart: restartMFARecoveryServer, TOTP: testTOTP, StoredState: storedState, LoginLabel: "6-digit code"})
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
