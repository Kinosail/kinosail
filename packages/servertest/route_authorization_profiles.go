package servertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// LoginRouteProfile retains the original session and enrollment rules for the route fixture.
func LoginRouteProfile(t *testing.T, handler http.Handler, name, password string) string {
	t.Helper()
	input := map[string]string{
		"name": name, "password": password, "device": "route authorization test",
	}
	if secured, ok := handler.(*RouteAuthorizationServer); ok && name == "Owner" {
		input["code"] = CurrentTOTP(secured.OwnerSecret)
	} else if secured, ok := handler.(*RouteAuthorizationServer); ok && name == "Viewer" && secured.ViewerSecret != "" {
		input["code"] = CurrentTOTP(secured.ViewerSecret)
	}
	response := RouteJSON(t, handler, "", http.MethodPost, "/api/v1/session", input)
	if response.Code != http.StatusCreated {
		t.Fatalf("API login = %d %q", response.Code, response.Body.String())
	}
	token := ResponseJSONToken(t, response)
	if name != "Owner" && strings.Contains(response.Body.String(), `"mfaEnrollmentRequired":true`) {
		CompleteRouteViewerEnrollment(t, handler, token)
	}
	return token
}

// CompleteRouteViewerEnrollment confirms the real Viewer factor and retains its test secret.
func CompleteRouteViewerEnrollment(t *testing.T, handler http.Handler, token string) {
	t.Helper()
	setup := RouteJSON(t, handler, token, http.MethodPost, "/api/v1/me/mfa/setup", map[string]any{})
	var enrollment struct {
		Secret string `json:"secret"`
	}
	if setup.Code != http.StatusCreated || json.Unmarshal(setup.Body.Bytes(), &enrollment) != nil || enrollment.Secret == "" {
		t.Fatalf("Viewer MFA setup = %d %q", setup.Code, setup.Body.String())
	}
	confirmed := RouteJSON(t, handler, token, http.MethodPut, "/api/v1/me/mfa", map[string]string{"code": CurrentTOTP(enrollment.Secret)})
	if confirmed.Code != http.StatusOK {
		t.Fatalf("Viewer MFA confirmation = %d %q", confirmed.Code, confirmed.Body.String())
	}
	if secured, ok := handler.(*RouteAuthorizationServer); ok {
		secured.ViewerSecret = enrollment.Secret
	}
}

// ResponseJSONToken retains strict token decoding and original failure diagnostics.
func ResponseJSONToken(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.Token == "" {
		t.Fatalf("session response = %d %q: %v", response.Code, response.Body.String(), err)
	}
	return result.Token
}
