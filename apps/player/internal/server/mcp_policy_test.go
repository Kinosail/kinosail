package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestMCPAPIRoutePolicy(t *testing.T) {
	servertest.AssertMCPAPIRoutePolicy(t, (mcpRoutePolicy{}).Allows, map[servertest.MCPPolicyInput]bool{
		{Pattern: "GET /api/v1/supporter/display", Write: true, Manage: true}:                            false,
		{Pattern: "PUT /api/v1/supporter/display", Write: true, Manage: true}:                            false,
		{Pattern: "GET /api/v1/supporter/certificates/living-standard.svg", Write: false, Manage: false}: false,
		{Pattern: "POST /api/v1/settings/trusted-https/test", Write: true, Manage: true}:                 true,
		{Pattern: "POST /api/v1/settings/trusted-https/validate", Write: true, Manage: true}:             true,
	})
}

func TestEveryVersionedAPIRouteHasExactlyOneMCPPolicy(t *testing.T) {
	servertest.AssertMCPRouteInventory(t, registeredRouteInventory(t), mcpReadRoutes, mcpWriteRoutes, mcpManageRoutes, mcpBlockedRoutes)
}

func mcpRequest(t *testing.T, handler http.Handler, method, token string, fields map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return servertest.MCPRequest(t, mcpProtocolVersion, handler, method, token, fields)
}
