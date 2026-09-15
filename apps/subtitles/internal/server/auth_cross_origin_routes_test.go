package server

import (
	"net/http"
	"strings"
	"testing"
)

func TestEveryMutatingRouteRejectsCrossOriginRequests(t *testing.T) {
	handler := New(Config{DataDir: t.TempDir(), RequireAuth: true})
	for _, pattern := range registeredRouteInventory(t) {
		method, _, _ := strings.Cut(pattern, " ")
		if method == http.MethodGet || method == http.MethodHead {
			continue
		}
		t.Run(pattern, func(t *testing.T) {
			response := exerciseRouteWithHeaders(t, handler, pattern, "", false, map[string]string{"Origin": "https://attacker.example"})
			if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "cross-origin request denied") {
				t.Fatalf("cross-origin mutation = %d %q", response.Code, response.Body.String())
			}
		})
	}
}
