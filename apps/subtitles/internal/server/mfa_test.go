package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestTOTPAndOneTimeRecoveryCodesGatePasswordSessions(t *testing.T) {
	servertest.AssertTOTPAndOneTimeRecoveryCodesGatePasswordSessions(t, servertest.MFARecoveryFixture{New: newMFARecoveryServer, Restart: restartMFARecoveryServer, TOTP: testTOTP, StoredState: storedState, LoginLabel: "6-digit code", NoJellyfin: true})
}

func TestDefaultMFADoesNotBlockOpenInitialServer(t *testing.T) {
	handler := server.New(server.Config{})
	response := apiCall(t, handler, "", http.MethodGet, "/api/v1/library", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("initial open library = %d %q", response.Code, response.Body.String())
	}
}

var enrollTestAPIFactor = servertest.EnrollTestAPIFactor

var testTOTP = servertest.TestTOTP
