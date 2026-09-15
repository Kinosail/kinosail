package servertest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// AssertPasskeyBeginRoutesFailClosedWhenConfigurationIsInvalid preserves the Player regression against the supplied app bindings.
func AssertPasskeyBeginRoutesFailClosedWhenConfigurationIsInvalid(t *testing.T, register, login http.HandlerFunc) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/passkeys/register/begin", nil)
	response := httptest.NewRecorder()
	register(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("invalid passkey registration = %d %q", response.Code, response.Body.String())
	}

	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/passkeys/login/begin", nil)
	response = httptest.NewRecorder()
	login(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("invalid passkey login = %d %q", response.Code, response.Body.String())
	}
}

// AssertAPIKeyPermissionsHonorScopeAndRouteClassification preserves the Player regression against the supplied app bindings.
func AssertAPIKeyPermissionsHonorScopeAndRouteClassification(t *testing.T, permits func(bool, []string, string, bool) bool, routeSet func(...string) map[string]bool) {
	if !permits(false, nil, "library", true) {
		t.Fatal("session profile lost an allowed permission")
	}
	if permits(true, []string{"admin"}, "library", true) {
		t.Fatal("API key used an unrelated scope")
	}
	if permits(true, []string{"library"}, "library", false) {
		t.Fatal("disabled permission was accepted")
	}
	routes := routeSet("GET /api/v1/library", "POST /api/v1/items/{id}/list")
	if !routes["GET /api/v1/library"] || routes["GET /api/v1/missing"] {
		t.Fatalf("route set = %#v", routes)
	}
}
