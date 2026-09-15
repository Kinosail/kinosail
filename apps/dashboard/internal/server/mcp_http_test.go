package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPTokenIssueReplacementAndRevocation(t *testing.T) {
	app := newTestApplication(t, Config{})
	session := app.setup(t)

	missingCSRF := app.request(t, http.MethodPost, "/api/v1/mcp-token", `{}`, &testSession{cookie: session.cookie})
	requireJSONError(t, missingCSRF, http.StatusForbidden, "request verification failed")

	first := issueMCPToken(t, app, session)
	if response := bearerRequest(t, app, http.MethodGet, "/api/v1/board", first); response.Code != http.StatusUnauthorized {
		t.Fatalf("MCP token used on API status = %d, want 401", response.Code)
	}
	if response := mcpInitializeRequest(t, app, first); response.Code != http.StatusOK {
		t.Fatalf("first MCP token status = %d, body = %s", response.Code, response.Body.String())
	}

	second := issueMCPToken(t, app, session)
	if first == second {
		t.Fatal("replacement MCP token did not change")
	}
	if response := mcpInitializeRequest(t, app, first); response.Code != http.StatusUnauthorized {
		t.Fatalf("replaced token status = %d, want 401", response.Code)
	}
	if response := mcpInitializeRequest(t, app, second); response.Code != http.StatusOK {
		t.Fatalf("replacement token status = %d, body = %s", response.Code, response.Body.String())
	}

	revoked := app.request(t, http.MethodDelete, "/api/v1/mcp-token", `{}`, &session)
	if revoked.Code != http.StatusNoContent {
		t.Fatalf("revoke status = %d, body = %s", revoked.Code, revoked.Body.String())
	}
	if response := mcpInitializeRequest(t, app, second); response.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token status = %d, want 401", response.Code)
	}
}

func TestMCPRejectsCookieAuthentication(t *testing.T) {
	app := newTestApplication(t, Config{})
	session := app.setup(t)
	response := app.request(t, http.MethodGet, "/mcp", "", &session)
	requireJSONError(t, response, http.StatusUnauthorized, "MCP requires a bearer token")
}

func TestMCPRejectsBrowserSessionUsedAsBearer(t *testing.T) {
	app := newTestApplication(t, Config{})
	session := app.setup(t)
	response := mcpInitializeRequest(t, app, session.cookie.Value)
	requireJSONError(t, response, http.StatusUnauthorized, "MCP requires a bearer token")
}

func TestMCPStreamableHTTPMutatesTheLiveBoard(t *testing.T) {
	app := newTestApplication(t, Config{})
	owner := app.setup(t)
	token := issueMCPToken(t, app, owner)
	httpServer := httptest.NewServer(app.handler)
	defer httpServer.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "MCP HTTP test", Version: "1"}, nil)
	httpClient := &http.Client{Transport: bearerTransport{token: token, next: http.DefaultTransport}}
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: httpServer.URL + "/mcp", HTTPClient: httpClient, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "add_application", Arguments: map[string]any{
		"name": "Router", "url": "http://router.lan", "checkEnabled": false, "expectedVersion": 1,
	}})
	if err != nil || result.IsError {
		t.Fatalf("HTTP MCP add failed: result=%#v err=%v", result, err)
	}
	if board := app.board.Snapshot(); board.Version != 2 || len(board.Apps) != 1 || board.Apps[0].Name != "Router" {
		t.Fatalf("live board after HTTP MCP add = %#v", board)
	}
}

type bearerTransport struct {
	token string
	next  http.RoundTripper
}

func (transport bearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	request.Header.Set("Authorization", "Bearer "+transport.token)
	return transport.next.RoundTrip(request)
}

func issueMCPToken(t *testing.T, app testApplication, session testSession) string {
	t.Helper()
	response := app.request(t, http.MethodPost, "/api/v1/mcp-token", `{}`, &session)
	if response.Code != http.StatusCreated {
		t.Fatalf("issue token status = %d, body = %s", response.Code, response.Body.String())
	}
	var output struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Token) != 64 {
		t.Fatalf("token length = %d, want 64", len(output.Token))
	}
	return output.Token
}

func bearerRequest(t *testing.T, app testApplication, method, path, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(""))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	app.handler.ServeHTTP(response, request)
	return response
}

func mcpInitializeRequest(t *testing.T, app testApplication, token string) *httptest.ResponseRecorder {
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	response := httptest.NewRecorder()
	app.handler.ServeHTTP(response, request)
	return response
}
