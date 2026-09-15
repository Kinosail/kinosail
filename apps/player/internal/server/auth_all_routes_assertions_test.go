package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func globalAuthenticationDenied(response *httptest.ResponseRecorder) bool {
	location := response.Header().Get("Location")
	return response.Code == http.StatusUnauthorized && strings.Contains(response.Body.String(), "authentication required") ||
		response.Code == http.StatusSeeOther && (location == "/setup" || location == "/login" || strings.HasPrefix(location, "/login?next="))
}

func routeAuthorizationDenied(pattern string, response *httptest.ResponseRecorder) bool {
	if pattern == "POST /logout" && response.Code == http.StatusSeeOther && response.Header().Get("Location") == "/login" {
		return false
	}
	body := response.Body.String()
	return globalAuthenticationDenied(response) || response.Code == http.StatusForbidden &&
		(strings.Contains(body, "Owner access required") || strings.Contains(body, "Viewer access is not available") || strings.Contains(body, "API key scope does not allow this request"))
}

func assertViewerPolicyDenied(t *testing.T, response *httptest.ResponseRecorder, policy string) {
	t.Helper()
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "Viewer access is not available") {
		t.Fatalf("%s policy did not deny route: %d location=%q body=%q", policy, response.Code, response.Header().Get("Location"), response.Body.String())
	}
}

func assertCapabilityRoute(t *testing.T, handler http.Handler, pattern string, withoutCapability *httptest.ResponseRecorder) {
	t.Helper()
	if strings.HasPrefix(pattern, "GET /Videos/") || pattern == "GET /Audio/{id}/{stream}" {
		if !globalAuthenticationDenied(withoutCapability) {
			t.Fatalf("Jellyfin playback route accepted a missing capability: %d %q", withoutCapability.Code, withoutCapability.Body.String())
		}
		withCapability := exerciseRoute(t, handler, pattern, "", true)
		if globalAuthenticationDenied(withCapability) || withCapability.Code != http.StatusNotFound {
			t.Fatalf("invalid Jellyfin capability did not reach the endpoint safely: %d %q", withCapability.Code, withCapability.Body.String())
		}
		return
	}
	if globalAuthenticationDenied(withoutCapability) || withoutCapability.Code != http.StatusNotFound {
		t.Fatalf("invalid DLNA capability did not reach the endpoint safely: %d %q", withoutCapability.Code, withoutCapability.Body.String())
	}
}
