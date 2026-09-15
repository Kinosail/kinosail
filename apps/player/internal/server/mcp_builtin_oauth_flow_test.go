package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

func TestBuiltInMCPOAuthWorksWithOfficialClientAndDCR(t *testing.T) { //nolint:cyclop,funlen // Official discovery, DCR, PKCE, token, and MCP behavior are one interoperability flow.
	profiles := newProfileStore("")
	profile := viewerProfile{ID: "viewer", Name: "Viewer", Libraries: []string{"all"}, Rating: "all"}
	profiles.profiles = []viewerProfile{profile}
	authentication := &authentication{profiles: profiles, audit: newAuditStore(t.Context(), t.TempDir(), nil)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/me", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, map[string]string{"viewer": currentViewer(request).Name}, http.StatusOK)
	})
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/oauth/authorize" {
			request = request.WithContext(withViewer(request.Context(), profile))
		}
		mux.ServeHTTP(writer, request)
	}))
	issuer := "http://" + server.Listener.Addr().String()
	connections := newMCPConnections(issuer, t.TempDir(), profiles, nil)
	registerMCPWithConnections(mux, MCPConfig{}, authentication, apiRouting(mux), connections)
	server.Start()
	t.Cleanup(server.Close)

	oauth, err := mcpauth.NewAuthorizationCodeHandler(&mcpauth.AuthorizationCodeHandlerConfig{
		DynamicClientRegistrationConfig: &mcpauth.DynamicClientRegistrationConfig{Metadata: &oauthex.ClientRegistrationMetadata{
			ClientName: "Official MCP client", RedirectURIs: []string{"http://127.0.0.1/callback"}, TokenEndpointAuthMethod: "none", GrantTypes: []string{"authorization_code", "refresh_token"}, ResponseTypes: []string{"code"},
		}},
		RedirectURL: "http://127.0.0.1/callback",
		AuthorizationCodeFetcher: func(ctx context.Context, arguments *mcpauth.AuthorizationArgs) (*mcpauth.AuthorizationResult, error) {
			client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			request, _ := http.NewRequestWithContext(ctx, http.MethodGet, arguments.URL, nil)
			response, err := client.Do(request)
			if err != nil {
				return nil, err
			}
			page, _ := io.ReadAll(response.Body)
			_ = response.Body.Close()
			pending := formValue(t, string(page), "request")
			request, _ = http.NewRequestWithContext(ctx, http.MethodPost, issuer+"/oauth/authorize", strings.NewReader(url.Values{"request": {pending}, "decision": {"allow"}, "scopes": {mcpReadScope}}.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response, err = client.Do(request)
			if err != nil {
				return nil, err
			}
			defer response.Body.Close()
			location, err := response.Location()
			if err != nil {
				return nil, err
			}
			return &mcpauth.AuthorizationResult{Code: location.Query().Get("code"), State: location.Query().Get("state"), Iss: location.Query().Get("iss")}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "Kinosail built-in OAuth test", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{Endpoint: issuer + "/mcp", OAuthHandler: oauth, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(t.Context(), nil)
	if err != nil || len(tools.Tools) != 3 {
		t.Fatalf("tools = %#v, %v", tools, err)
	}
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "read_api", Arguments: map[string]any{"path": "/api/v1/me"}})
	encoded, _ := json.Marshal(result.StructuredContent)
	if err != nil || result.IsError || !strings.Contains(string(encoded), `"viewer":"Viewer"`) {
		t.Fatalf("read_api = %s, %v", encoded, err)
	}
}
