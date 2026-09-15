package mcpgateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type failingAPI struct {
	calls int
	err   error
}

func (api *failingAPI) Pattern(request *http.Request) string {
	return request.Method + " " + request.URL.Path
}

func (api *failingAPI) Invoke(http.ResponseWriter, *http.Request, Principal, string, bool) error {
	api.calls++
	return api.err
}

func TestRegisterBuiltInOAuthAndRouteInventory(t *testing.T) {
	principals := &testPrincipals{values: map[string]Principal{}}
	api := &testAPI{respond: func(http.ResponseWriter, *http.Request) {}}
	connections := testConnections("https://kino.test", principals, &memoryState{})
	mux := http.NewServeMux()
	adapter, err := Register(mux, testGatewayConfig(OAuthConfig{}, principals, api, testRoutes{}), connections)
	if err != nil || adapter == nil || adapter.config.ResourceURL != connections.resource || len(Patterns()) != 10 {
		t.Fatalf("built-in registration = %#v, %v patterns=%d", adapter, err, len(Patterns()))
	}
}

func TestCallAPIFailuresStopBeforeApplicationSideEffects(t *testing.T) {
	owner := Principal{ID: "owner", Name: "Owner", Owner: true}
	api := &failingAPI{err: errors.New("invoke failed")}
	routes := testRoutes{ReadAccess: {"GET /api/v1/test": true}}
	adapter := newGateway(testGatewayConfig(OAuthConfig{}, &testPrincipals{values: map[string]Principal{"owner": owner}}, api, routes))
	if _, err := adapter.callAPI(t.Context(), http.MethodGet, "/api/v1/test", nil, ReadAccess); err == nil || api.calls != 0 {
		t.Fatalf("missing principal = %v calls=%d", err, api.calls)
	}
	ctx := withStdioPrincipal(t.Context(), owner)
	if _, err := adapter.callAPI(ctx, "bad\nmethod", "/api/v1/test", nil, ReadAccess); err == nil || api.calls != 0 {
		t.Fatalf("invalid request method = %v calls=%d", err, api.calls)
	}
	if _, err := adapter.callAPI(ctx, http.MethodGet, "/api/v1/test", nil, ReadAccess); !errors.Is(err, api.err) || api.calls != 1 {
		t.Fatalf("invoke failure = %v calls=%d", err, api.calls)
	}
}

func TestAPIPathBodyAndResponseBoundaryErrors(t *testing.T) {
	if _, err := mcpAPIPath("https://example.test/api/v1"); err == nil {
		t.Fatal("absolute API path was accepted")
	}
	if _, err := mcpAPIBody(func() {}); err == nil {
		t.Fatal("unsupported API body was encoded")
	}
	empty := &mcpAPIRecorder{header: make(http.Header), body: mcpBoundedBuffer{limit: 32}}
	output, err := mcpAPIResponse("GET /api/v1/test", empty)
	if err != nil || output.Status != http.StatusOK || output.Body != nil || empty.statusCode() != http.StatusOK {
		t.Fatalf("empty API response = %#v, %v", output, err)
	}
	nonJSON := &mcpAPIRecorder{header: http.Header{"Content-Type": []string{"text/plain"}}, body: mcpBoundedBuffer{limit: 32}}
	_, _ = nonJSON.Write([]byte("plain"))
	if _, err := mcpAPIResponse("GET /api/v1/test", nonJSON); err == nil {
		t.Fatal("non-JSON API response was accepted")
	}
	malformed := &mcpAPIRecorder{header: http.Header{"Content-Type": []string{"application/json"}}, body: mcpBoundedBuffer{limit: 32}}
	_, _ = malformed.Write([]byte(`{`))
	if _, err := mcpAPIResponse("GET /api/v1/test", malformed); err == nil {
		t.Fatal("malformed JSON API response was accepted")
	}
}

func TestProtocolMiddlewarePropagatesHandlerError(t *testing.T) {
	want := errors.New("handler failed")
	handler := modernMCPResults(func(context.Context, string, mcp.Request) (mcp.Result, error) { return nil, want })
	if _, err := handler(t.Context(), "test", nil); !errors.Is(err, want) {
		t.Fatalf("middleware error = %v", err)
	}
}

func TestToolValidationAndRecommendationFailurePaths(t *testing.T) {
	owner := Principal{ID: "owner", Name: "Owner", Owner: true}
	api := &failingAPI{err: errors.New("history failed")}
	routes := testRoutes{ReadAccess: {"GET /api/v1/history": true, "GET /api/v1/library": true}}
	adapter := newGateway(testGatewayConfig(OAuthConfig{}, &testPrincipals{values: map[string]Principal{"owner": owner}}, api, routes))
	if _, _, err := adapter.manageAPI(t.Context(), nil, mcpManageAPIInput{Method: "OPTIONS"}); err == nil {
		t.Fatal("invalid management method was accepted")
	}
	if _, _, err := adapter.searchMedia(t.Context(), nil, mcpMediaInput{View: "invalid"}); err == nil {
		t.Fatal("invalid search input was accepted")
	}
	if _, _, err := adapter.recommendationContext(t.Context(), nil, mcpMediaInput{View: "invalid"}); err == nil {
		t.Fatal("invalid recommendation input was accepted")
	}
	if _, _, err := adapter.recommendationContext(withStdioPrincipal(t.Context(), owner), nil, mcpMediaInput{}); !errors.Is(err, api.err) || api.calls != 1 {
		t.Fatalf("history failure = %v calls=%d", err, api.calls)
	}
}

func TestIntrospectionFailuresDoNotAttributePrincipal(t *testing.T) {
	principals := &testPrincipals{values: map[string]Principal{}, oidc: map[string]Principal{}}
	adapter := &Gateway{config: OAuthConfig{IntrospectionURL: ":", ClientID: "id", ClientSecret: "secret"}, principals: principals}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil)
	if _, err := adapter.verifyToken(t.Context(), "token", request); err == nil || principals.attributed != (Principal{}) {
		t.Fatalf("invalid introspection URL = %v attributed=%#v", err, principals.attributed)
	}
	adapter.config.IntrospectionURL = "http://127.0.0.1:1"
	if _, err := adapter.verifyToken(t.Context(), "token", request); err == nil || principals.attributed != (Principal{}) {
		t.Fatalf("unavailable introspection = %v attributed=%#v", err, principals.attributed)
	}
	status, body := http.StatusUnauthorized, `{}`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(body))
	}))
	defer server.Close()
	adapter.config.IntrospectionURL = server.URL
	if _, err := adapter.verifyToken(t.Context(), "token", request); err == nil {
		t.Fatal("non-OK introspection was accepted")
	}
	status, body = http.StatusOK, `{`
	if _, err := adapter.verifyToken(t.Context(), "token", request); err == nil {
		t.Fatal("malformed introspection was accepted")
	}
	status, body = http.StatusOK, `{"active":false}`
	if _, err := adapter.verifyToken(t.Context(), "token", request); err == nil {
		t.Fatal("inactive introspection was accepted")
	}
	status, body = http.StatusOK, string(mustJSON(t, map[string]any{"active": true, "scope": ReadScope, "exp": time.Now().Add(time.Hour).Unix(), "sub": "missing", "aud": "https://kino.test/mcp"}))
	adapter.config.ResourceURL, adapter.config.AuthorizationServer = "https://kino.test/mcp", "https://identity.test"
	if _, err := adapter.verifyToken(t.Context(), "token", request); err == nil || principals.attributed != (Principal{}) {
		t.Fatalf("unknown introspection principal = %v attributed=%#v", err, principals.attributed)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestIntrospectionValueBoundsAndMetadataURLFailure(t *testing.T) {
	resource := "https://kino.test/mcp"
	if validMCPToken(true, 1, strings.Repeat("s", 257), ReadScope, json.RawMessage(`"`+resource+`"`), resource) {
		t.Fatal("oversized subject was accepted")
	}
	if validMCPToken(true, 1, "subject", strings.Repeat("scope ", 65), json.RawMessage(`"`+resource+`"`), resource) {
		t.Fatal("excessive scopes were accepted")
	}
	if validMCPToken(true, 1, "subject", strings.Repeat("x", 129), json.RawMessage(`"`+resource+`"`), resource) {
		t.Fatal("oversized scope was accepted")
	}
	if mcpAudience(json.RawMessage(`"`+resource+`"`), "") || validMCPAudiences([]string{strings.Repeat("x", 2049)}) || mcpMetadataURL(":") != "" {
		t.Fatal("introspection bounds changed")
	}
}
