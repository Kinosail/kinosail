package servertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// SetupRouteAuthorization preserves Owner enrollment and enabled Jellyfin routes.
func SetupRouteAuthorization(t *testing.T, handler http.Handler, responseToken func(*testing.T, *httptest.ResponseRecorder) string) (string, string) {
	t.Helper()
	response := RouteJSON(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{
		"name": "Owner", "password": "owner-password", "device": "route authorization test", "totp": true,
	})
	if response.Code != http.StatusCreated {
		t.Fatalf("API setup = %d %q", response.Code, response.Body.String())
	}
	token := responseToken(t, response)
	var setup struct {
		TOTP struct {
			Secret string `json:"secret"`
		} `json:"totp"`
	}
	if json.Unmarshal(response.Body.Bytes(), &setup) != nil || setup.TOTP.Secret == "" {
		t.Fatalf("API setup factor = %q", response.Body.String())
	}
	confirmed := RouteJSON(t, handler, token, http.MethodPut, "/api/v1/me/mfa", map[string]string{"code": CurrentTOTP(setup.TOTP.Secret)})
	if confirmed.Code != http.StatusOK {
		t.Fatalf("confirm API setup factor = %d %q", confirmed.Code, confirmed.Body.String())
	}
	if enabled := RouteJSON(t, handler, token, http.MethodPut, "/api/v1/settings/jellyfin", map[string]bool{"enabled": true}); enabled.Code != http.StatusOK {
		t.Fatalf("enable Jellyfin routes = %d %q", enabled.Code, enabled.Body.String())
	}
	return setup.TOTP.Secret, token
}
