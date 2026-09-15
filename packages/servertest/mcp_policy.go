package servertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/mcpgateway"
)

// MCPPolicyInput describes route access flags checked by the app policy.
type MCPPolicyInput struct {
	Pattern       string
	Write, Manage bool
}

// AssertMCPAPIRoutePolicy checks common policy cases and explicit app additions.
func AssertMCPAPIRoutePolicy(t *testing.T, allows func(string, mcpgateway.AccessClass) bool, additional map[MCPPolicyInput]bool) {
	t.Helper()
	cases := map[MCPPolicyInput]bool{
		{"GET /api/v1/library", false, false}:                       true,
		{"GET /api/v1/settings", false, false}:                      false,
		{"GET /api/v1/settings", true, true}:                        true,
		{"GET /api/v1/updates", true, true}:                         true,
		{"GET /api/v1/activity/export", false, false}:               false,
		{"GET /api/v1/agent-connections/certificate", false, false}: false,
		{"GET /api/v1/backup", false, false}:                        false,
		{"GET /api/v1/downloads/{id}/file", false, false}:           false,
		{"PUT /api/v1/items/{id}/progress", true, false}:            true,
		{"PUT /api/v1/me/language", true, false}:                    true,
		{"PUT /api/v1/settings/server", true, false}:                false,
		{"PUT /api/v1/settings/server", true, true}:                 true,
		{"PUT /api/v1/settings/updates", true, true}:                true,
		{"POST /api/v1/updates", true, true}:                        true,
		{"POST /api/v1/tasks/{task}", true, true}:                   true,
		{"PUT /api/v1/profiles/{id}/password", true, true}:          false,
		{"POST /api/v1/passkeys/login/begin", true, false}:          false,
		{"DELETE /api/v1/remote-access/wireguard", true, true}:      false,
	}
	for input, allowed := range additional {
		cases[input] = allowed
	}
	for input, allowed := range cases {
		access := mcpgateway.ReadAccess
		if input.Write {
			access = mcpgateway.WriteAccess
		}
		if input.Manage {
			access = mcpgateway.ManageAccess
		}
		if actual := allows(input.Pattern, access); actual != allowed {
			t.Errorf("mcpAPIRouteAllowed(%q, %v, %v) = %v, want %v", input.Pattern, input.Write, input.Manage, actual, allowed)
		}
	}
}

// AssertMCPRouteInventory checks bidirectional, exclusive classification of every API route.
func AssertMCPRouteInventory(t *testing.T, inventory []string, classes ...map[string]bool) {
	t.Helper()
	registered := make(map[string]bool)
	for _, pattern := range inventory {
		if !strings.Contains(pattern, " /api/v1") {
			continue
		}
		registered[pattern] = true
		classifications := countMCPClassifications(pattern, classes)
		if classifications != 1 {
			t.Errorf("%q has %d MCP classifications, want exactly one", pattern, classifications)
		}
	}
	for _, routes := range classes {
		for pattern := range routes {
			if !registered[pattern] {
				t.Errorf("obsolete MCP classification %q has no registered API route", pattern)
			}
		}
	}
}

func countMCPClassifications(pattern string, classes []map[string]bool) int {
	count := 0
	for _, routes := range classes {
		if routes[pattern] {
			count++
		}
	}
	return count
}

// MCPRequest sends the canonical MCP request envelope to the real handler.
func MCPRequest(t *testing.T, protocolVersion string, handler http.Handler, method, token string, fields map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	params := map[string]any{"_meta": map[string]any{
		"io.modelcontextprotocol/protocolVersion":    protocolVersion,
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
	}}
	for key, value := range fields {
		params[key] = value
	}
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("MCP-Protocol-Version", protocolVersion)
	request.Header.Set("Mcp-Method", method)
	if name, _ := params["name"].(string); name != "" {
		request.Header.Set("Mcp-Name", name)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
