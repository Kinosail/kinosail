//go:build !windows

package mcpgateway

import (
	"context"
	"net"
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestOpenStdioConnectionRechecksOwnerAuthority(t *testing.T) {
	for _, change := range []string{"deleted", "demoted", "replaced", "unavailable"} {
		t.Run(change, func(t *testing.T) {
			owner := Principal{ID: "owner", Name: "Owner", Owner: true, Revision: 4}
			principals := &testPrincipals{values: map[string]Principal{owner.ID: owner}}
			api := &testAPI{respond: func(writer http.ResponseWriter, _ *http.Request) { writeJSON(writer, map[string]bool{"ok": true}, 200) }}
			adapter := newGateway(testGatewayConfig(OAuthConfig{}, principals, api, testRoutes{ManageAccess: {"POST /api/v1/tasks/scan": true}}))
			session := connectAuditStdio(t, adapter, principals, owner.ID)
			params := &mcp.CallToolParams{Name: "manage_api", Arguments: map[string]any{"method": "POST", "path": "/api/v1/tasks/scan"}}
			if result, err := session.CallTool(t.Context(), params); err != nil || result.IsError {
				t.Fatalf("valid Owner rejected: %v %v", result, err)
			}
			principals.mu.Lock()
			switch change {
			case "deleted":
				delete(principals.values, owner.ID)
			case "demoted":
				owner.Owner = false
				principals.values[owner.ID] = owner
			case "replaced":
				owner.Revision++
				principals.values[owner.ID] = owner
			case "unavailable":
				principals.err = context.Canceled
			}
			principals.mu.Unlock()
			if result, err := session.CallTool(t.Context(), params); err == nil && !result.IsError {
				t.Fatalf("stale Owner retained authority: %v", result)
			}
			assertSingleAuditAPICall(t, api)
		})
	}
}

func connectAuditStdio(t *testing.T, adapter *Gateway, principals *testPrincipals, profileID string) *mcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	dataDir := t.TempDir()
	if err := StartStdioHost(ctx, dataDir, adapter, principals); err != nil {
		t.Fatal(err)
	}
	relay, err := OpenStdioRelay(ctx, dataDir, profileID)
	if err != nil {
		t.Fatal(err)
	}
	connection, err := net.FileConn(relay)
	_ = relay.Close()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	session, err := mcp.NewClient(&mcp.Implementation{Name: "audit", Version: "1"}, nil).Connect(ctx, &mcp.IOTransport{Reader: connection, Writer: connection}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })

	return session
}

func assertSingleAuditAPICall(t *testing.T, api *testAPI) {
	t.Helper()
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.calls != 1 {
		t.Fatalf("rejected Owner caused %d API calls", api.calls)
	}
}
