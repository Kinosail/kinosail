package server

import (
	"net/http"
	"strings"
	"testing"
)

func TestEveryProtectedRouteEnforcesBrowserSessionRolesAndRevocation(t *testing.T) { //nolint:cyclop,gocognit // Every route repeats the positive and negative cookie cases.
	handler, ownerToken := newRouteAuthorizationServer(t)
	createRouteProfile(t, handler, ownerToken, map[string]any{
		"name": "Viewer", "password": "viewer-password", "rating": "all", "libraries": []string{"all"},
		"downloads": true, "transcode": true, "remote": true,
	})
	owner := loginRouteCookie(t, handler, "Owner", "owner-password")
	viewer := loginRouteCookie(t, handler, "Viewer", "viewer-password")
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
			viewerResponse := exerciseRouteWithCookie(t, handler, pattern, viewer)
			if ownerOnlyRoutes[pattern] {
				if viewerResponse.Code != http.StatusForbidden || !strings.Contains(viewerResponse.Body.String(), "Owner access required") {
					t.Fatalf("Viewer cookie reached Owner route: %d %q", viewerResponse.Code, viewerResponse.Body.String())
				}
			} else if routeAuthorizationDenied(pattern, viewerResponse) {
				t.Fatalf("Viewer cookie was denied: %d location=%q body=%q", viewerResponse.Code, viewerResponse.Header().Get("Location"), viewerResponse.Body.String())
			}

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
		switch pattern {
		case "DELETE /api/v1/session", "POST /logout", "POST /Sessions/Logout":
			owner = loginRouteCookie(t, handler, "Owner", "owner-password")
			viewer = loginRouteCookie(t, handler, "Viewer", "viewer-password")
		case "DELETE /api/v1/sessions", "POST /settings/sessions/revoke":
			viewer = loginRouteCookie(t, handler, "Viewer", "viewer-password")
		}
	}
}
