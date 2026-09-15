package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail-dashboard/internal/dashboard"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCoverageMCPToolsUseLiveBoard(t *testing.T) {
	app := newTestApplication(t, Config{})
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusOK) }))
	t.Cleanup(provider.Close)
	created, _, err := app.board.Create(t.Context(), dashboard.CreateInput{Name: "Media", URL: provider.URL, CheckEnabled: true, ExpectedVersion: 1}, "Owner")
	if err != nil {
		t.Fatal(err)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := newMCPServer(app.board, app.prober).Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "Dashboard coverage", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	for _, call := range []struct {
		name string
		args map[string]any
	}{
		{"list_application_catalog", map[string]any{}},
		{"update_application", map[string]any{"id": created.ID, "name": "Changed", "expectedVersion": 2}},
		{"reorder_applications", map[string]any{"ids": []string{created.ID}, "expectedVersion": 3}},
		{"check_application", map[string]any{"id": created.ID}},
		{"remove_application", map[string]any{"id": created.ID, "expectedVersion": 4, "confirm": true}},
	} {
		result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: call.name, Arguments: call.args})
		if err != nil || result.IsError {
			t.Fatalf("%s = %+v, %v", call.name, result, err)
		}
	}
	assertCoverageMCPBoard(t, app.board.Snapshot())
}

func assertCoverageMCPBoard(t *testing.T, got dashboard.Snapshot) {
	t.Helper()
	if got.Version != 5 || len(got.Apps) != 0 || len(got.Removed) != 1 || got.Removed[0].App.Name != "Changed" {
		t.Fatalf("MCP board = %+v", got)
	}
}
