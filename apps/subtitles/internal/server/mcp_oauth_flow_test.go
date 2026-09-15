package server

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

func TestMCPOAuthAuthorizationCodeFlowWithOfficialClient(t *testing.T) { //nolint:cyclop,funlen,gocognit // The full browserless OAuth redirect and MCP exchange are one interoperability fixture.
	var resourceURL string
	var authorizationURL string
	var mu sync.Mutex
	codeChallenge := ""
	authorization := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/.well-known/oauth-authorization-server":
			writeJSON(writer, map[string]any{
				"issuer":                                         authorizationURL,
				"authorization_endpoint":                         authorizationURL + "/authorize",
				"token_endpoint":                                 authorizationURL + "/token",
				"grant_types_supported":                          []string{"authorization_code"},
				"code_challenge_methods_supported":               []string{"S256"},
				"token_endpoint_auth_methods_supported":          []string{"none"},
				"authorization_response_iss_parameter_supported": true,
			}, http.StatusOK)
		case "/authorize":
			query := request.URL.Query()
			if query.Get("response_type") != "code" || query.Get("client_id") != "agent" || query.Get("resource") != resourceURL || query.Get("code_challenge_method") != "S256" {
				http.Error(writer, "invalid authorization request", http.StatusBadRequest)
				return
			}
			mu.Lock()
			codeChallenge = query.Get("code_challenge")
			mu.Unlock()
			redirect, _ := url.Parse(query.Get("redirect_uri"))
			values := redirect.Query()
			values.Set("code", "authorization-code")
			values.Set("state", query.Get("state"))
			values.Set("iss", authorizationURL)
			redirect.RawQuery = values.Encode()
			http.Redirect(writer, request, redirect.String(), http.StatusFound) //nolint:gosec // The test authorization server validates this fixture redirect above.
		case "/token":
			_ = request.ParseForm()
			verifier := request.Form.Get("code_verifier")
			digest := sha256.Sum256([]byte(verifier))
			mu.Lock()
			validChallenge := codeChallenge != "" && codeChallenge == base64.RawURLEncoding.EncodeToString(digest[:])
			mu.Unlock()
			if request.Form.Get("grant_type") != "authorization_code" || request.Form.Get("code") != "authorization-code" || request.Form.Get("client_id") != "agent" || request.Form.Get("resource") != resourceURL || !validChallenge {
				http.Error(writer, "invalid token request", http.StatusBadRequest)
				return
			}
			writeJSON(writer, map[string]any{"access_token": "oauth-access-token", "token_type": "Bearer", "expires_in": 3600, "scope": mcpReadScope}, http.StatusOK) //nolint:gosec // A non-secret fixture token is required for the flow.
		case "/introspect":
			if id, secret, ok := request.BasicAuth(); !ok || id != "resource" || secret != "secret" {
				http.Error(writer, "invalid resource credentials", http.StatusUnauthorized)
				return
			}
			_ = request.ParseForm()
			if request.Form.Get("token") != "oauth-access-token" {
				writeJSON(writer, map[string]bool{"active": false}, http.StatusOK)
				return
			}
			writeJSON(writer, map[string]any{"active": true, "scope": mcpReadScope, "exp": time.Now().Add(time.Hour).Unix(), "sub": "subject", "aud": resourceURL}, http.StatusOK)
		default:
			http.NotFound(writer, request)
		}
	}))
	authorizationURL = authorization.URL
	t.Cleanup(authorization.Close)

	profiles := newProfileStore("")
	profiles.profiles = []viewerProfile{{ID: "viewer", Name: "Viewer", Libraries: []string{"all"}, Rating: "all", OIDCIssuer: authorization.URL, OIDCSubject: "subject"}}
	authentication := &authentication{profiles: profiles, audit: newAuditStore(t.Context(), t.TempDir(), nil)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/me", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, map[string]string{"viewer": currentViewer(request).Name}, http.StatusOK)
	})
	resource := httptest.NewUnstartedServer(mux)
	resourceURL = "http://" + resource.Listener.Addr().String() + "/mcp"
	registerMCPWithConnections(mux, MCPConfig{ResourceURL: resourceURL, AuthorizationServer: authorization.URL, IntrospectionURL: authorization.URL + "/introspect", ClientID: "resource", ClientSecret: "secret"}, authentication, apiRouting(mux), nil)
	resource.Start()
	t.Cleanup(resource.Close)

	oauth, err := mcpauth.NewAuthorizationCodeHandler(&mcpauth.AuthorizationCodeHandlerConfig{
		PreregisteredClient: &oauthex.ClientCredentials{ClientID: "agent", Issuer: authorization.URL},
		RedirectURL:         "http://127.0.0.1/callback",
		AuthorizationCodeFetcher: func(ctx context.Context, args *mcpauth.AuthorizationArgs) (*mcpauth.AuthorizationResult, error) {
			client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			authorizationRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, args.URL, nil)
			if err != nil {
				return nil, err
			}
			response, err := client.Do(authorizationRequest)
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
	client := mcp.NewClient(&mcp.Implementation{Name: "Kinosail OAuth test agent", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{Endpoint: resourceURL, OAuthHandler: oauth, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if got := session.InitializeResult().ProtocolVersion; got != mcpProtocolVersion {
		t.Fatalf("protocol version = %q", got)
	}
	tools, err := session.ListTools(t.Context(), nil)
	if err != nil || len(tools.Tools) != 3 {
		t.Fatalf("tools = %#v, %v", tools, err)
	}
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "read_api", Arguments: map[string]any{"path": "/api/v1/me"}})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(result.StructuredContent)
	if result.IsError || !strings.Contains(string(encoded), `"viewer":"Viewer"`) {
		t.Fatalf("read_api = %s", encoded)
	}
}
