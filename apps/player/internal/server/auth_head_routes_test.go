package server

import (
	"net/http"
	"strings"
	"testing"
)

func TestEveryGetEndpointAppliesTheSameAuthorizationToHead(t *testing.T) { //nolint:cyclop,funlen,gocognit // Go implicitly maps HEAD to GET, so every authorization dimension is repeated here.
	handler, owner := newRouteAuthorizationServer(t)
	createRouteProfile(t, handler, owner, map[string]any{
		"name": "Viewer", "password": "viewer-password", "rating": "all", "libraries": []string{"all"},
		"downloads": true, "transcode": true, "remote": true,
	})
	viewer := loginRouteProfile(t, handler, "Viewer", "viewer-password")
	keys := make(map[string]string)
	for _, scope := range []string{"library", "write", "stream", "download", "admin"} {
		keys[scope] = createRouteAPIKey(t, handler, owner, scope)
	}

	for _, pattern := range registeredRouteInventory(t) {
		if !strings.HasPrefix(pattern, "GET ") {
			continue
		}
		t.Run(pattern, func(t *testing.T) {
			anonymous := exerciseRouteAsMethod(t, handler, pattern, http.MethodHead, "", false, nil)
			if explicitlyAnonymousRoutes[pattern] {
				if globalAuthenticationDenied(anonymous) {
					t.Fatalf("anonymous HEAD was blocked: %d %q", anonymous.Code, anonymous.Body.String())
				}
				return
			}
			if localAnonymousJellyfinRoutes[pattern] {
				if globalAuthenticationDenied(anonymous) {
					t.Fatalf("local Jellyfin compatibility HEAD was blocked: %d %q", anonymous.Code, anonymous.Body.String())
				}
				return
			}
			if explicitCapabilityRoutes[pattern] {
				if strings.HasPrefix(pattern, "GET /Videos/") || pattern == "GET /Audio/{id}/{stream}" {
					if !globalAuthenticationDenied(anonymous) {
						t.Fatalf("HEAD accepted a missing playback capability: %d %q", anonymous.Code, anonymous.Body.String())
					}
				} else if globalAuthenticationDenied(anonymous) {
					t.Fatalf("HEAD blocked DLNA capability routing: %d %q", anonymous.Code, anonymous.Body.String())
				}
				return
			}
			if !globalAuthenticationDenied(anonymous) {
				t.Fatalf("anonymous HEAD reached protected route: %d %q", anonymous.Code, anonymous.Body.String())
			}

			ownerResponse := exerciseRouteAsMethod(t, handler, pattern, http.MethodHead, owner, false, nil)
			if routeAuthorizationDenied(pattern, ownerResponse) {
				t.Fatalf("Owner HEAD was denied: %d %q", ownerResponse.Code, ownerResponse.Body.String())
			}
			viewerResponse := exerciseRouteAsMethod(t, handler, pattern, http.MethodHead, viewer, false, nil)
			if ownerOnlyRoutes[pattern] {
				if viewerResponse.Code != http.StatusForbidden || !strings.Contains(viewerResponse.Body.String(), "Owner access required") {
					t.Fatalf("Viewer HEAD reached Owner route: %d %q", viewerResponse.Code, viewerResponse.Body.String())
				}
			} else if routeAuthorizationDenied(pattern, viewerResponse) {
				t.Fatalf("Viewer HEAD was denied: %d %q", viewerResponse.Code, viewerResponse.Body.String())
			}

			expected := expectedAPIKeyScopes(pattern)
			for scope, key := range keys {
				response := exerciseRouteAsMethod(t, handler, pattern, http.MethodHead, key, false, nil)
				if expected[scope] {
					if routeAuthorizationDenied(pattern, response) {
						t.Errorf("%s API key HEAD was denied: %d %q", scope, response.Code, response.Body.String())
					}
				} else if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "API key scope does not allow this request") {
					t.Errorf("%s API key HEAD reached route: %d %q", scope, response.Code, response.Body.String())
				}
			}
		})
	}
}
