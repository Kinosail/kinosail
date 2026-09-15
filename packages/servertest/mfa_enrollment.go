package servertest

import (
	"net/http"
	"testing"
	"time"
)

// EnrollTestAPIFactor retains the app helper enrollment and confirmation assertions.
func EnrollTestAPIFactor(t *testing.T, handler http.Handler, token string) {
	t.Helper()
	setup := APICall(t, handler, token, http.MethodPost, "/api/v1/me/mfa/setup", map[string]any{})
	var enrollment struct {
		Secret string `json:"secret"`
	}
	MustJSON(t, setup, &enrollment)
	if setup.Code != http.StatusCreated || enrollment.Secret == "" {
		t.Fatalf("MFA setup = %d %q", setup.Code, setup.Body.String())
	}
	confirmed := APICall(t, handler, token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": TestTOTP(t, enrollment.Secret, time.Now())})
	if confirmed.Code != http.StatusOK {
		t.Fatalf("MFA confirmation = %d %q", confirmed.Code, confirmed.Body.String())
	}
}
