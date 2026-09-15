package servertest

import (
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/mcpgateway"
	"github.com/MikeO7/kinosail/packages/routeinventory"
	"github.com/MikeO7/kinosail/packages/scim"
)

// RegisteredRouteInventory discovers the exact original app and shared route inventory.
func RegisteredRouteInventory(t *testing.T) []string {
	t.Helper()
	routes, err := routeinventory.Discover(".", "../../../../packages", scim.Patterns(), mcpgateway.Patterns())
	if err != nil {
		t.Fatal(err)
	}
	return routes
}

// ConcreteRoutePath fills the original route placeholders for authorization checks.
func ConcreteRoutePath(pattern string) string {
	if pattern == "/{$}" {
		return "/"
	}
	return strings.NewReplacer(
		"{file...}", "index.m3u8", "{asset...}", "missing", "{stream...}", "360p/index.m3u8", "{source}", "source", "{stream}", "stream",
		"{language}", "en", "{person}", "0", "{second}", "0", "{track}", "0", "{index}", "0",
		"{token}", "invalid", "{type}", "Primary", "{task}", "scan", "{code}", "invalid", "{key}", "server.name",
		"{user}", "missing", "{name}", "missing", "{id}", "missing",
	).Replace(pattern)
}

// ExpectedRouteAPIKeyScopes classifies only the supplied app policy maps.
func ExpectedRouteAPIKeyScopes(pattern string, sessionOnly map[string]bool, scopes map[string]map[string]bool) map[string]bool {
	if sessionOnly[pattern] {
		return nil
	}
	if pattern == "GET /api/v1/me" || pattern == "PUT /api/v1/me/language" {
		return map[string]bool{"library": true, "admin": true}
	}
	for scope, routes := range scopes {
		if routes[pattern] {
			return map[string]bool{scope: true}
		}
	}
	if strings.Contains(pattern, " /api/v1/") {
		return map[string]bool{"admin": true}
	}
	return nil
}
