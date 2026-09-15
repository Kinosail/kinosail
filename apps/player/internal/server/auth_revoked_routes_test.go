package server

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestEveryProtectedRouteRejectsARevokedAPIKey(t *testing.T) {
	handler, owner := newRouteAuthorizationServer(t)
	revoked := createRouteAPIKey(t, handler, owner, "library,write,stream,download,admin")
	response := routeJSON(t, handler, owner, http.MethodGet, "/api/v1/api-keys", nil)
	var result struct {
		Keys []struct {
			ID string `json:"id"`
		} `json:"keys"`
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &result) != nil || len(result.Keys) != 1 {
		t.Fatalf("list API keys = %d %q", response.Code, response.Body.String())
	}
	response = routeJSON(t, handler, owner, http.MethodDelete, "/api/v1/api-keys/"+result.Keys[0].ID, nil)
	if response.Code != http.StatusNoContent {
		t.Fatalf("revoke API key = %d %q", response.Code, response.Body.String())
	}

	for _, pattern := range registeredRouteInventory(t) {
		if explicitlyAnonymousRoutes[pattern] || explicitCapabilityRoutes[pattern] {
			continue
		}
		t.Run(pattern, func(t *testing.T) {
			response := exerciseRoute(t, handler, pattern, revoked, false)
			if !globalAuthenticationDenied(response) {
				t.Fatalf("revoked API key reached route: %d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
			}
		})
	}
}
