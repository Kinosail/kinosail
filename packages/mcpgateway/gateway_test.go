package mcpgateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type testPrincipalKey struct{}

type testPrincipals struct {
	mu         sync.Mutex
	values     map[string]Principal
	oidc       map[string]Principal
	recent     bool
	err        error
	attributed Principal
}

func (store *testPrincipals) Current(request *http.Request) Principal {
	value, _ := request.Context().Value(testPrincipalKey{}).(Principal)
	return value
}

func (store *testPrincipals) ByID(id string) (Principal, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	value, found := store.values[id]
	return value, found
}

func (store *testPrincipals) ByOIDC(issuer, subject string) (Principal, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	value, found := store.oidc[issuer+"\x00"+subject]
	return value, found
}

func (store *testPrincipals) Allowed(principal Principal, _ *http.Request, _ time.Time) bool {
	value, found := store.ByID(principal.ID)
	return found && value == principal
}

func (store *testPrincipals) RecentlyAuthenticated(*http.Request, time.Duration) bool {
	return store.recent
}

func (store *testPrincipals) Owners() ([]Principal, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.err != nil {
		return nil, store.err
	}
	owners := make([]Principal, 0)
	for _, value := range store.values {
		if value.Owner {
			owners = append(owners, value)
		}
	}
	return owners, nil
}

func (store *testPrincipals) Attribute(_ *http.Request, principal Principal) {
	store.mu.Lock()
	store.attributed = principal
	store.mu.Unlock()
}

type testAPI struct {
	mu      sync.Mutex
	calls   int
	actor   string
	manage  bool
	last    Principal
	respond func(http.ResponseWriter, *http.Request)
}

func (api *testAPI) Pattern(request *http.Request) string {
	return request.Method + " " + request.URL.Path
}

func (api *testAPI) Invoke(writer http.ResponseWriter, request *http.Request, principal Principal, actor string, manage bool) error {
	api.mu.Lock()
	api.calls++
	api.actor, api.manage, api.last = actor, manage, principal
	api.mu.Unlock()
	api.respond(writer, request)
	return nil
}

type testRoutes map[AccessClass]map[string]bool

func (routes testRoutes) Allows(pattern string, access AccessClass) bool {
	return routes[access][pattern]
}

func testGatewayConfig(oauth OAuthConfig, principals PrincipalRepository, api APIInvoker, routes RoutePolicy) GatewayConfig {
	return GatewayConfig{
		OAuth: oauth, Principals: principals, API: api, Routes: routes,
	}
}

func mcpRequest(t *testing.T, handler http.Handler, method string, fields map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	params := map[string]any{"_meta": map[string]any{
		"io.modelcontextprotocol/protocolVersion":    ProtocolVersion,
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
	}}
	for key, value := range fields {
		params[key] = value
	}
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	request.Header.Set("Mcp-Method", method)
	if name, _ := params["name"].(string); name != "" {
		request.Header.Set("Mcp-Name", name)
	}
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestGatewayUsesPlayerToolsAndFailClosedPolicies(t *testing.T) { //nolint:cyclop // The score of 12 remains below the repository ceiling of 22 for one fail-closed policy matrix.
	scope, audience := ReadScope, "http://localhost/mcp"
	issuer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		id, secret, basic := request.BasicAuth()
		if request.Method != http.MethodPost || request.URL.Path != "/introspect" || !basic || id != "resource" || secret != "secret" {
			t.Fatal("invalid introspection request")
		}
		_ = request.ParseForm()
		writeJSON(writer, map[string]any{"active": true, "scope": scope, "exp": time.Now().Add(time.Hour).Unix(), "sub": "subject", "aud": audience}, http.StatusOK)
	}))
	defer issuer.Close()
	viewer := Principal{ID: "viewer", Name: "Viewer"}
	principals := &testPrincipals{values: map[string]Principal{"viewer": viewer}, oidc: map[string]Principal{issuer.URL + "\x00subject": viewer}}
	api := &testAPI{respond: func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/library":
			if request.URL.Query().Get("q") == "oversized-response" {
				writeJSON(writer, map[string]string{"value": strings.Repeat("x", 5<<20)}, http.StatusOK)
				return
			}
			writeJSON(writer, map[string]any{"items": []any{map[string]string{"title": request.URL.Query().Get("q")}}}, http.StatusOK)
		case "/api/v1/history":
			writeJSON(writer, map[string]any{"items": []any{map[string]string{"id": "watched"}}}, http.StatusOK)
		case "/api/v1/metrics":
			writer.Header().Set("Content-Type", "text/plain")
			_, _ = writer.Write([]byte("kinosail_health 1\n"))
		default:
			writeJSON(writer, map[string]any{"ok": true}, http.StatusOK)
		}
	}}
	routes := testRoutes{
		ReadAccess:   {"GET /api/v1/library": true, "GET /api/v1/history": true},
		WriteAccess:  {"POST /api/v1/playlists": true, "PUT /api/v1/me/language": true},
		ManageAccess: {"GET /api/v1/metrics": true, "PUT /api/v1/settings/server": true},
	}
	mux := http.NewServeMux()
	config := OAuthConfig{ResourceURL: audience, AuthorizationServer: issuer.URL, IntrospectionURL: issuer.URL + "/introspect", ClientID: "resource", ClientSecret: "secret"}
	if _, err := Register(mux, testGatewayConfig(config, principals, api, routes), nil); err != nil {
		t.Fatal(err)
	}

	assertGatewayProtocol(t, mux)
	before := assertReadTools(t, mux, api)
	scope = ReadScope + " " + WriteScope
	assertWriteTools(t, mux, api, viewer, before)
	owner := Principal{ID: "viewer", Name: "Viewer", Owner: true}
	principals.values["viewer"] = owner
	principals.oidc[issuer.URL+"\x00subject"] = owner
	scope = ReadScope + " " + ManageScope
	assertManageTools(t, mux, api)
}

func TestGatewayRejectsInvalidConfigurationAndProtocolWithoutEffects(t *testing.T) {
	calls := 0
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ })
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader("{}"))
	request.Header.Set("MCP-Protocol-Version", "unknown")
	response := httptest.NewRecorder()
	modernMCP(next).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || calls != 0 || !strings.Contains(response.Body.String(), ProtocolVersion) {
		t.Fatalf("protocol = %d calls=%d %q", response.Code, calls, response.Body.String())
	}
	if _, err := Register(nil, GatewayConfig{}, nil); err == nil {
		t.Fatal("nil router was accepted")
	}
	mux := http.NewServeMux()
	if _, err := Register(mux, GatewayConfig{}, nil); err == nil {
		t.Fatal("missing adapters were accepted")
	}
	principals := &testPrincipals{values: map[string]Principal{}}
	api := &testAPI{respond: func(http.ResponseWriter, *http.Request) {}}
	if _, err := Register(mux, testGatewayConfig(OAuthConfig{}, principals, api, testRoutes{}), nil); err == nil {
		t.Fatal("missing OAuth authority was accepted")
	}
}
