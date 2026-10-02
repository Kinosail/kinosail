package mcpgateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func auditGateway(t *testing.T, api *testAPI) *http.ServeMux {
	t.Helper()
	owner := Principal{ID: "owner", Name: "Owner", Owner: true}
	principals := &testPrincipals{values: map[string]Principal{owner.ID: owner}}
	connections := testConnections("https://kino.test", principals, &memoryState{})
	connections.grants["grant"] = mcpOAuthGrant{
		ID: "grant", ProfileID: owner.ID, AccessHash: secretHash("token"),
		AccessExpires: time.Now().Add(time.Hour).Unix(), Scopes: []string{ReadScope, WriteScope, ManageScope},
	}
	mux := http.NewServeMux()
	routes := testRoutes{
		ReadAccess:  {"GET /api/v1/library": true, "GET /api/v1/history": true},
		WriteAccess: {"POST /api/v1/playlists": true}, ManageAccess: {"POST /api/v1/tasks/scan": true},
	}
	if _, err := Register(mux, testGatewayConfig(OAuthConfig{}, principals, api, routes), connections); err != nil {
		t.Fatal(err)
	}
	return mux
}

func TestMCPAPIFailuresAreToolErrorsWithStructuredStatus(t *testing.T) {
	status := http.StatusOK
	api := &testAPI{respond: func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, map[string]string{"error": "operation rejected"}, status)
	}}
	mux := auditGateway(t, api)
	for _, tool := range []struct {
		name      string
		arguments map[string]any
	}{
		{"read_api", map[string]any{"path": "/api/v1/library"}},
		{"search_media", map[string]any{}},
		{"recommendation_context", map[string]any{}},
		{"write_api", map[string]any{"method": "POST", "path": "/api/v1/playlists"}},
		{"create_playlist", map[string]any{"name": "Test", "ids": []string{"movie"}}},
		{"manage_api", map[string]any{"method": "POST", "path": "/api/v1/tasks/scan"}},
	} {
		for _, code := range []int{200, 400, 403, 404, 409, 422, 429, 500, 503} {
			t.Run(tool.name+"/"+strconv.Itoa(code), func(t *testing.T) {
				status = code
				response := mcpRequest(t, mux, "tools/call", map[string]any{"name": tool.name, "arguments": tool.arguments})
				var envelope struct {
					Result struct {
						IsError           bool            `json:"isError"`
						StructuredContent json.RawMessage `json:"structuredContent"`
					}
				}
				if json.Unmarshal(response.Body.Bytes(), &envelope) != nil || response.Code != http.StatusOK || envelope.Result.IsError != (code >= 400) || !strings.Contains(string(envelope.Result.StructuredContent), `"status":`+strconv.Itoa(code)) {
					t.Fatalf("tool status = %d %s", response.Code, response.Body.String())
				}
			})
		}
	}
}

func TestMCPRequestAndToolRatesStopBeforeEffects(t *testing.T) {
	for _, method := range []string{"server/discover", "tools/call"} {
		t.Run(method, func(t *testing.T) {
			api := &testAPI{respond: func(writer http.ResponseWriter, _ *http.Request) { writeJSON(writer, map[string]bool{"ok": true}, 200) }}
			mux := auditGateway(t, api)
			fields := map[string]any{"name": "read_api", "arguments": map[string]any{"path": "/api/v1/library"}}
			if method == "server/discover" {
				fields = nil
			}
			for index := range 121 {
				handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					if method == "tools/call" {
						request.RemoteAddr = "192.0.2." + strconv.Itoa(index+1) + ":1234"
					}
					mux.ServeHTTP(writer, request)
				})
				response := mcpRequest(t, handler, method, fields)
				assertAuditRateResponse(t, response, method, index, api.calls)
			}
		})
	}
}

func TestOAuthAuthorizationRejectsMalformedAndOversizedQueries(t *testing.T) {
	owner := Principal{ID: "owner", Owner: true}
	store := &memoryState{}
	connections := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{owner.ID: owner}}, store)
	connections.clients["agent"] = mcpOAuthClient{ID: "agent", Name: "Agent", RedirectURIs: []string{"http://127.0.0.1/callback"}}
	query := url.Values{"response_type": {"code"}, "client_id": {"agent"}, "redirect_uri": {"http://127.0.0.1/callback"}, "resource": {connections.resource}, "code_challenge": {strings.Repeat("A", 43)}, "code_challenge_method": {"S256"}}.Encode()
	mux := http.NewServeMux()
	connections.RegisterOAuth(mux)
	for _, suffix := range []string{"&ignored=%zz", "&ignored=x;y", strings.Repeat("&", 16<<10)} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, principalRequest(t, http.MethodGet, "/oauth/authorize?"+query+suffix, strings.NewReader(""), owner))
		if response.Code != 400 || len(connections.pending) != 0 || len(connections.codes) != 0 || store.saves != 0 {
			t.Fatalf("invalid query created authority: %d pending=%d", response.Code, len(connections.pending))
		}
	}
}

func TestIntrospectionDoesNotForwardBearerCredentialsThroughRedirects(t *testing.T) {
	forwarded := 0
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		forwarded++
		writeJSON(writer, map[string]bool{"active": false}, 200)
	}))
	defer target.Close()
	issuer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusTemporaryRedirect)
	}))
	defer issuer.Close()
	adapter := &Gateway{config: OAuthConfig{IntrospectionURL: issuer.URL, ClientID: "id", ClientSecret: "secret"}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil)
	if _, err := adapter.verifyToken(t.Context(), "bearer-secret", request); err == nil || forwarded != 0 {
		t.Fatalf("introspection redirect followed: forwarded=%d err=%v", forwarded, err)
	}
}

func TestIntrospectionRejectsAmbiguousClaimsWithoutAttribution(t *testing.T) {
	owner := Principal{ID: "owner", Owner: true}
	principals := &testPrincipals{values: map[string]Principal{owner.ID: owner}, oidc: map[string]Principal{"https://identity.test\x00subject": owner}}
	body := `{"active":false,"active":true,"scope":"kinosail.read","sub":"subject","exp":2000000000,"aud":"https://kino.test/mcp"}`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte(body)) }))
	defer server.Close()
	adapter := &Gateway{principals: principals, config: OAuthConfig{IntrospectionURL: server.URL, AuthorizationServer: "https://identity.test", ResourceURL: "https://kino.test/mcp"}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil)
	if info, err := adapter.verifyToken(t.Context(), "token", request); err == nil || info != nil || principals.attributed != (Principal{}) {
		t.Fatal("ambiguous introspection created authority")
	}
	body = `{"active":true,"scope":"kinosail.read","sub":"subject","exp":2000000000,"aud":"https://kino.test/mcp","token_type":"Bearer","client_id":"agent","extension":{"valid":true}}`
	if info, err := adapter.verifyToken(t.Context(), "token", request); err != nil || info == nil {
		t.Fatalf("valid extensible introspection rejected: %v", err)
	}
}

func TestMissingProtocolHeaderIsHeaderMismatch(t *testing.T) {
	called := false
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader("{}"))
	modernMCP(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).ServeHTTP(response, request)
	if response.Code != 400 || !strings.Contains(response.Body.String(), `"code":-32020`) || called {
		t.Fatalf("missing version = %d %s", response.Code, response.Body.String())
	}
}

func TestOAuthPublicCredentialEndpointsHaveBoundedRequestRates(t *testing.T) {
	store := &memoryState{}
	connections := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{}}, store)
	mux := http.NewServeMux()
	connections.RegisterOAuth(mux)
	for index := range 121 {
		request := tokenRequest(t, url.Values{"token": {"unknown"}, "client_id": {"unknown"}})
		request.URL.Path = "/oauth/revoke"
		if index%2 == 1 {
			request.URL.Path = "/oauth/token"
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if index == 120 && (response.Code != 429 || response.Header().Get("Retry-After") == "") {
			t.Fatalf("OAuth flood accepted: %d", response.Code)
		}
		if store.saves != 0 {
			t.Fatal("rejected credential requests changed authority")
		}
	}
}

func TestOAuthApprovalCodeCapacityIsBounded(t *testing.T) {
	owner := Principal{ID: "owner", Owner: true}
	connections := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{owner.ID: owner}}, &memoryState{})
	for index := range 128 {
		connections.codes[strconv.Itoa(index)] = mcpOAuthCode{Expires: time.Now().Add(time.Minute).Unix()}
	}
	connections.pending["key:request"] = mcpOAuthRequest{Client: mcpOAuthClient{ID: "agent"}, ProfileID: owner.ID, RedirectURI: "http://127.0.0.1/callback", Scopes: []string{ReadScope}, Expires: time.Now().Add(time.Minute).Unix()}
	request := principalRequest(t, http.MethodPost, "/oauth/authorize", strings.NewReader(url.Values{"request": {"request"}, "decision": {"allow"}, "scopes": {ReadScope}}.Encode()), owner)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	connections.authorize(response, request)
	if response.Code != 429 || len(connections.codes) != 128 {
		t.Fatalf("code capacity exceeded: %d codes=%d", response.Code, len(connections.codes))
	}
}

func TestMCPDiscoveryMatchesStaticToolsAndUnsupportedPrimitives(t *testing.T) {
	mux := auditGateway(t, &testAPI{respond: func(http.ResponseWriter, *http.Request) {}})
	response := mcpRequest(t, mux, "server/discover", nil)
	var envelope struct {
		Result struct{ Capabilities map[string]json.RawMessage }
	}
	if json.Unmarshal(response.Body.Bytes(), &envelope) != nil {
		t.Fatal("invalid discovery")
	}
	var tools struct {
		ListChanged bool `json:"listChanged"`
	}
	if json.Unmarshal(envelope.Result.Capabilities["tools"], &tools) != nil || tools.ListChanged {
		t.Fatal("static tools advertise change subscriptions")
	}
	for _, method := range []string{"prompts/list", "resources/list", "resources/templates/list"} {
		response = mcpRequest(t, mux, method, nil)
		if response.Code != 404 || !strings.Contains(response.Body.String(), `"code":-32601`) {
			t.Fatalf("undeclared primitive was served: %s %d %s", method, response.Code, response.Body.String())
		}
	}
}

func TestMCPVersionErrorsPreserveRequestIDs(t *testing.T) {
	for _, version := range []string{"", "2025-11-25", "2099-01-01"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":"request-42","method":"tools/list"}`))
		request.Header.Set("MCP-Protocol-Version", version)
		response := httptest.NewRecorder()
		modernMCP(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unsupported protocol was dispatched") })).ServeHTTP(response, request)
		if response.Code != 400 || !strings.Contains(response.Body.String(), `"id":"request-42"`) {
			t.Fatalf("lost request ID: %d %s", response.Code, response.Body.String())
		}
	}
}

func TestMCPVersionErrorsDoNotEchoInvalidOrAmbiguousIDs(t *testing.T) {
	for _, body := range []string{`{"id":{}}`, `{"id":1.5}`, `{"id":"a","id":"b"}`, `{"id":"a"}{}`, `{"id":`} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(body))
		response := httptest.NewRecorder()
		modernMCP(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("invalid envelope dispatched") })).ServeHTTP(response, request)
		var envelope map[string]json.RawMessage
		if json.Unmarshal(response.Body.Bytes(), &envelope) != nil || envelope["id"] != nil {
			t.Fatalf("invalid ID echoed: %s", response.Body.String())
		}
	}
}

func assertAuditRateResponse(t *testing.T, response *httptest.ResponseRecorder, method string, index, calls int) {
	t.Helper()
	if index < 120 {
		if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `"isError":true`) {
			t.Fatalf("early limit at %d: %s", index, response.Body.String())
		}
		return
	}
	if method == "server/discover" {
		if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") == "" {
			t.Fatalf("request flood accepted: %d", response.Code)
		}
		return
	}
	if !strings.Contains(response.Body.String(), `"isError":true`) || calls != 120 {
		t.Fatalf("tool flood accepted: calls=%d %s", calls, response.Body.String())
	}
}

func TestMCPVersionErrorsPreserveBoundedLongStringID(t *testing.T) {
	id := strings.Repeat("x", 2049)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(`{"id":"`+id+`"}`))
	response := httptest.NewRecorder()
	modernMCP(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unsupported protocol dispatched") })).ServeHTTP(response, request)
	var envelope struct{ ID string }
	if json.Unmarshal(response.Body.Bytes(), &envelope) != nil || envelope.ID != id {
		t.Fatal("readable request ID was lost")
	}
}

func TestMCPRejectsMalformedAPIQueryBeforeInvocation(t *testing.T) {
	calls := 0
	mux := auditGateway(t, &testAPI{respond: func(http.ResponseWriter, *http.Request) { calls++ }})
	for _, path := range []string{"/api/v1/library?query=%", "/api/v1/library?query=%zz", "/api/v1/library?query=a;b"} {
		response := mcpRequest(t, mux, "tools/call", map[string]any{"name": "read_api", "arguments": map[string]any{"path": path}})
		if !strings.Contains(response.Body.String(), `"isError":true`) || calls != 0 {
			t.Fatalf("malformed query invoked API: %s %s", path, response.Body.String())
		}
	}
}
