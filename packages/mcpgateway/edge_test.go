package mcpgateway

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

func TestToolInputsAndOutputsStayBounded(t *testing.T) {
	for name, input := range map[string]mcpMediaInput{
		"view": {View: "invalid"}, "sort": {Sort: "invalid"}, "query": {Query: strings.Repeat("x", 513)},
		"small limit": {Limit: -1}, "large limit": {Limit: 201},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := mcpMediaPath(input); err == nil {
				t.Fatal("invalid media input was accepted")
			}
		})
	}
	path, limit, err := mcpMediaPath(mcpMediaInput{})
	if err != nil || limit != 50 || !strings.Contains(path, "limit=50") {
		t.Fatalf("default media path = %q %d %v", path, limit, err)
	}
	output := mcpAPIOutput{Body: map[string]any{"items": []any{1, 2, 3}}}
	limited := limitMCPItems(output, 2)
	if len(limited.Body.(map[string]any)["items"].([]any)) != 2 {
		t.Fatalf("limited output = %#v", limited)
	}
	if got := limitMCPItems(mcpAPIOutput{Body: "text"}, 1); got.Body != "text" {
		t.Fatalf("non-list output = %#v", got)
	}
}

func TestOAuthValidationRejectsAmbiguousExternalData(t *testing.T) { //nolint:funlen // Tables cover bounded OAuth metadata and token contracts.
	resource := "https://kino.test/mcp"
	for name, raw := range map[string]json.RawMessage{
		"empty": nil, "unknown": json.RawMessage(`{"bad":true}`), "empty list": json.RawMessage(`[]`),
		"too many":  json.RawMessage(`[` + strings.Repeat(`"a",`, 16) + `"a"]`),
		"oversized": json.RawMessage(`"` + strings.Repeat("x", 2049) + `"`),
	} {
		t.Run(name, func(t *testing.T) {
			if mcpAudience(raw, resource) {
				t.Fatal("invalid audience was accepted")
			}
		})
	}
	if !mcpAudience(json.RawMessage(`[`+`"other",`+`"https://kino.test/mcp"`+`]`), resource) {
		t.Fatal("valid audience list was rejected")
	}
	if validMCPToken(false, time.Now().Unix(), "subject", ReadScope, json.RawMessage(`"`+resource+`"`), resource) || validMCPToken(true, 0, "subject", ReadScope, json.RawMessage(`"`+resource+`"`), resource) || validMCPToken(true, 1, "", ReadScope, json.RawMessage(`"`+resource+`"`), resource) {
		t.Fatal("invalid token facts were accepted")
	}
	var target struct{ OK bool }
	if httpguard.DecodeJSON(strings.NewReader(`{"OK":true,"extra":true}`), 64, &target, true) == nil || httpguard.DecodeJSON(strings.NewReader(strings.Repeat("x", 65)), 64, &target, false) == nil || httpguard.DecodeJSON(strings.NewReader(`{"OK":true} trailing`), 64, &target, false) == nil {
		t.Fatal("invalid external JSON was accepted")
	}
}

func TestClientMetadataDocumentAndRedirectRules(t *testing.T) {
	var metadataURL string
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, map[string]any{
			"client_id": metadataURL, "client_name": "Codex", "redirect_uris": []string{"http://127.0.0.1/callback"},
			"token_endpoint_auth_method": "none", "grant_types": []string{"authorization_code"}, "response_types": []string{"code"},
		}, http.StatusOK)
	}))
	defer server.Close()
	metadataURL = server.URL + "/client.json"
	connections := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{}}, &memoryState{})
	connections.client = server.Client()
	client, err := connections.clientFor(t.Context(), metadataURL)
	if err != nil || client.ID != metadataURL || client.Name != "Codex" {
		t.Fatalf("metadata client = %#v %v", client, err)
	}
	connections.clients[client.ID] = client
	if cached, err := connections.clientFor(t.Context(), client.ID); err != nil || cached.ID != client.ID {
		t.Fatalf("cached client = %#v %v", cached, err)
	}
	if !redirectAllowed([]string{"http://127.0.0.1/callback"}, "http://127.0.0.1:43123/callback") || redirectAllowed(client.RedirectURIs, "https://attacker.example/callback") {
		t.Fatal("loopback redirect rules changed")
	}
	delete(connections.clients, client.ID)
	connections.client = publicMetadataHTTPClient(time.Second)
	if _, err := connections.clientFor(t.Context(), metadataURL); err == nil {
		t.Fatal("private metadata address was accepted")
	}
}

func TestConnectionConfigurationAndMetadataValidation(t *testing.T) { //nolint:funlen // Configuration and registration metadata stay closed.
	connections := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{}}, &memoryState{})
	config := OAuthConfig{ResourceURL: "https://resource.test/mcp", AuthorizationServer: "https://identity.test", IntrospectionURL: "https://identity.test/introspect", ClientID: "id", ClientSecret: "secret"}
	connections.Configure(config)
	if !connections.External() || connections.Issuer() != config.AuthorizationServer || connections.Resource() != config.ResourceURL {
		t.Fatalf("external configuration = %v %q %q", connections.External(), connections.Issuer(), connections.Resource())
	}
	for name, metadata := range map[string]oauthex.ClientRegistrationMetadata{
		"missing name":     {RedirectURIs: []string{"http://127.0.0.1/callback"}},
		"missing redirect": {ClientName: "Agent"},
		"bad auth":         {ClientName: "Agent", RedirectURIs: []string{"http://127.0.0.1/callback"}, TokenEndpointAuthMethod: "client_secret_basic"},
		"bad grant":        {ClientName: "Agent", RedirectURIs: []string{"http://127.0.0.1/callback"}, GrantTypes: []string{"client_credentials"}},
		"bad response":     {ClientName: "Agent", RedirectURIs: []string{"http://127.0.0.1/callback"}, ResponseTypes: []string{"token"}},
		"bad application":  {ClientName: "Agent", RedirectURIs: []string{"http://127.0.0.1/callback"}, ApplicationType: "service"},
		"bad contact":      {ClientName: "Agent", RedirectURIs: []string{"http://127.0.0.1/callback"}, Contacts: []string{""}},
		"bad scope":        {ClientName: "Agent", RedirectURIs: []string{"http://127.0.0.1/callback"}, Scope: ManageScope},
	} {
		t.Run(name, func(t *testing.T) {
			if validateMCPClientMetadata(&metadata) == nil {
				t.Fatal("invalid metadata was accepted")
			}
		})
	}
	valid := oauthex.ClientRegistrationMetadata{ClientName: " Agent ", RedirectURIs: []string{"http://localhost/callback"}}
	if err := validateMCPClientMetadata(&valid); err != nil || valid.ClientName != "Agent" || valid.TokenEndpointAuthMethod != "none" {
		t.Fatalf("valid metadata = %#v %v", valid, err)
	}
}

func TestOAuthAuthorizationDenialConsumesPendingState(t *testing.T) {
	viewer := Principal{ID: "viewer", Name: "Viewer"}
	principals := &testPrincipals{values: map[string]Principal{"viewer": viewer}}
	connections := testConnections("https://kino.test", principals, &memoryState{})
	connections.pending["key:request"] = mcpOAuthRequest{Client: mcpOAuthClient{ID: "agent"}, RedirectURI: "http://127.0.0.1/callback", State: "state", Scopes: []string{ReadScope}, ProfileID: viewer.ID, Expires: time.Now().Add(time.Minute).Unix()}
	mux := http.NewServeMux()
	connections.RegisterOAuth(mux)
	request := principalRequest(t, http.MethodPost, "/oauth/authorize", strings.NewReader(url.Values{"request": {"request"}, "decision": {"deny"}, "scopes": {ReadScope}}.Encode()), viewer)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusFound || len(connections.pending) != 0 || !strings.Contains(response.Header().Get("Location"), "error=access_denied") {
		t.Fatalf("denial = %d pending=%d location=%q", response.Code, len(connections.pending), response.Header().Get("Location"))
	}
	unauthorized := httptest.NewRecorder()
	mux.ServeHTTP(unauthorized, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/oauth/authorize", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized approval = %d", unauthorized.Code)
	}
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		t.Fatal(err)
	}
}
