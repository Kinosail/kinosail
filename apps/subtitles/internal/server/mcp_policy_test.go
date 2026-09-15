package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestMCPAPIRoutePolicy(t *testing.T) {
	servertest.AssertMCPAPIRoutePolicy(t, (mcpRoutePolicy{}).Allows, nil)
}

func TestEveryVersionedAPIRouteHasExactlyOneMCPPolicy(t *testing.T) {
	servertest.AssertMCPRouteInventory(t, registeredRouteInventory(t), mcpReadRoutes, mcpWriteRoutes, mcpManageRoutes, mcpBlockedRoutes)
}

func mcpRequest(t *testing.T, handler http.Handler, method, token string, fields map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return servertest.MCPRequest(t, mcpProtocolVersion, handler, method, token, fields)
}
