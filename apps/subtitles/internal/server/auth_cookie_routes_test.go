package server

import (
	"net/http"
	"testing"
)

func TestEveryProtectedRouteEnforcesBrowserSessionOwnershipAndRevocation(t *testing.T) {
	handler, _ := newRouteAuthorizationServer(t)
	owner := loginRouteCookie(t, handler, "Owner", "owner-password")
	invalid := &http.Cookie{Name: "__Host-kinosail_session", Value: "revoked-or-malformed", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}
	revoked := loginRouteCookie(t, handler, "Owner", "owner-password")
	if response := exerciseRouteWithCookie(t, handler, "DELETE /api/v1/session", revoked); response.Code != http.StatusNoContent {
		t.Fatalf("revoke browser session = %d %q", response.Code, response.Body.String())
	}

	for _, pattern := range registeredRouteInventory(t) {
		if explicitlyAnonymousRoutes[pattern] || explicitCapabilityRoutes[pattern] {
			continue
		}
		t.Run(pattern, func(t *testing.T) {
			ownerResponse := exerciseRouteWithCookie(t, handler, pattern, owner)
			if routeAuthorizationDenied(pattern, ownerResponse) {
				t.Fatalf("Owner cookie was denied: %d location=%q body=%q", ownerResponse.Code, ownerResponse.Header().Get("Location"), ownerResponse.Body.String())
			}

			invalidResponse := exerciseRouteWithCookie(t, handler, pattern, invalid)
			if !globalAuthenticationDenied(invalidResponse) {
				t.Fatalf("invalid cookie reached route: %d location=%q body=%q", invalidResponse.Code, invalidResponse.Header().Get("Location"), invalidResponse.Body.String())
			}
			revokedResponse := exerciseRouteWithCookie(t, handler, pattern, revoked)
			if !globalAuthenticationDenied(revokedResponse) {
				t.Fatalf("revoked cookie reached route: %d location=%q body=%q", revokedResponse.Code, revokedResponse.Header().Get("Location"), revokedResponse.Body.String())
			}
		})
		if pattern == "DELETE /api/v1/session" || pattern == "POST /logout" {
			owner = loginRouteCookie(t, handler, "Owner", "owner-password")
		}
	}
}
