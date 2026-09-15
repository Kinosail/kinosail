package mcpgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

const (
	mcpAPIPathLimit     = 2048
	mcpAPIBodyLimit     = 1 << 20
	mcpAPIResponseLimit = 4 << 20
)

// Gateway provides the shared MCP protocol and API-tool implementation.
type Gateway struct {
	config     OAuthConfig
	principals PrincipalRepository
	api        APIInvoker
	routes     RoutePolicy
	servers    [4]*mcp.Server
}

type mcpReadAPIInput struct {
	Path string `json:"path" jsonschema:"Versioned Kinosail JSON API path or /api/v1/metrics, including any query string; start with /api/v1"`
}

type mcpWriteAPIInput struct {
	Method string `json:"method" jsonschema:"HTTP method: POST, PUT, PATCH, or DELETE"`
	Path   string `json:"path" jsonschema:"Versioned Kinosail API path, including any query string; start with /api/v1"`
	Body   any    `json:"body,omitempty" jsonschema:"JSON request body when the API operation requires one"`
}

type mcpManageAPIInput struct {
	Method string `json:"method" jsonschema:"HTTP method: GET, POST, PUT, PATCH, or DELETE"`
	Path   string `json:"path" jsonschema:"Approved Owner API path, including any query string; start with /api/v1"`
	Body   any    `json:"body,omitempty" jsonschema:"JSON request body when the API operation requires one"`
}

type mcpAPIOutput struct {
	Status int `json:"status"`
	Body   any `json:"body,omitempty"`
}

// Register installs the protected resource and MCP transport routes.
func Register(mux *http.ServeMux, config GatewayConfig, connections *Connections) (*Gateway, error) {
	if mux == nil {
		return nil, errors.New("MCP gateway requires an HTTP router")
	}
	if err := config.validate(); err != nil {
		return nil, err
	}
	var verifier mcpauth.TokenVerifier
	if !config.OAuth.Configured() {
		if connections == nil {
			return nil, errors.New("MCP gateway requires OAuth configuration or built-in connections")
		}
		config.OAuth.ResourceURL, config.OAuth.AuthorizationServer = connections.resource, connections.issuer
		connections.RegisterOAuth(mux)
		verifier = connections.VerifyToken
	}
	adapter := newGateway(config)
	if verifier == nil {
		verifier = adapter.verifyToken
	}
	metadataURL := mcpMetadataURL(config.OAuth.ResourceURL)
	metadata := &oauthex.ProtectedResourceMetadata{
		Resource:               config.OAuth.ResourceURL,
		AuthorizationServers:   []string{config.OAuth.AuthorizationServer},
		ScopesSupported:        []string{ReadScope, WriteScope, ManageScope},
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "Kinosail",
	}
	mux.Handle("GET /.well-known/oauth-protected-resource/mcp", mcpauth.ProtectedResourceMetadataHandler(metadata))
	mux.Handle("OPTIONS /.well-known/oauth-protected-resource/mcp", mcpauth.ProtectedResourceMetadataHandler(metadata))
	mux.HandleFunc("GET /mcp", mcpMethodNotAllowed)
	mux.HandleFunc("DELETE /mcp", mcpMethodNotAllowed)
	handler := mcp.NewStreamableHTTPHandler(adapter.server, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, PropagateRequestCancellation: true})
	handlerWithAuth := mcpauth.RequireBearerToken(verifier, &mcpauth.RequireBearerTokenOptions{Scopes: []string{ReadScope}, ResourceMetadataURL: metadataURL})(modernMCP(handler))
	mux.Handle("POST /mcp", http.NewCrossOriginProtection().Handler(handlerWithAuth))
	return adapter, nil
}

func newGateway(config GatewayConfig) *Gateway {
	adapter := &Gateway{config: config.OAuth, principals: config.Principals, api: config.API, routes: config.Routes}
	for access := range adapter.servers {
		adapter.servers[access] = adapter.newServer(access&1 != 0, access&2 != 0)
	}
	return adapter
}

func (adapter *Gateway) newServer(writable, manageable bool) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "Kinosail", Version: "1"}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{},
		Instructions: "Search media and use recommendation_context before creating playlists. read_api exposes Viewer library and viewing context. manage_api exposes approved Owner operational reads, setup, and organization routes. Media bytes, credentials, sessions, and interactive identity operations are not exposed.",
	})
	server.AddReceivingMiddleware(modernMCPResults)
	closed := false
	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_api",
		Title:       "Read Kinosail API",
		Description: "Read JSON library and viewing-context responses from Kinosail's versioned API as the authenticated Viewer or Owner.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &closed},
	}, adapter.readAPI)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_media",
		Title:       "Search Kinosail media",
		Description: "Find visible media by title, year, genre, people, director, studio, artist, album, show, viewing state, and media section.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &closed},
	}, adapter.searchMedia)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "recommendation_context",
		Title:       "Prepare personal recommendations",
		Description: "Return recent viewing history and matching candidates so the connected agent can explain and rank personalized recommendations.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &closed},
	}, adapter.recommendationContext)
	if writable {
		destructive := true
		mcp.AddTool(server, &mcp.Tool{
			Name:        "write_api",
			Title:       "Update Kinosail",
			Description: "Call Viewer-owned JSON API mutations such as progress, My List, playlists, and language.",
			Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closed},
		}, adapter.writeAPI)
		mcp.AddTool(server, &mcp.Tool{
			Name:        "create_playlist",
			Title:       "Create an AI-curated playlist",
			Description: "Create one Viewer-owned playlist from an ordered list of media IDs selected by the connected agent.",
			Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closed},
		}, adapter.createPlaylist)
	}
	if manageable {
		destructive := true
		mcp.AddTool(server, &mcp.Tool{
			Name:        "manage_api",
			Title:       "Manage Kinosail",
			Description: "As an Owner, read operational status, organize metadata and collections, configure playback and libraries, run scans and maintenance, test the transcoder, and manage backups through approved /api/v1 operations.",
			Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closed},
		}, adapter.manageAPI)
	}
	return server
}

func (adapter *Gateway) server(request *http.Request) *mcp.Server {
	access := 0
	if token := mcpauth.TokenInfoFromContext(request.Context()); token != nil {
		if contains(token.Scopes, WriteScope) {
			access |= 1
		}
		if profile, found := adapter.principals.ByID(token.UserID); found && profile.Owner && contains(token.Scopes, ManageScope) {
			access |= 2
		}
	}
	return adapter.servers[access]
}

func (adapter *Gateway) readAPI(ctx context.Context, _ *mcp.CallToolRequest, input mcpReadAPIInput) (*mcp.CallToolResult, mcpAPIOutput, error) {
	output, err := adapter.callAPI(ctx, http.MethodGet, input.Path, nil, ReadAccess)
	return nil, output, err
}

func (adapter *Gateway) writeAPI(ctx context.Context, _ *mcp.CallToolRequest, input mcpWriteAPIInput) (*mcp.CallToolResult, mcpAPIOutput, error) {
	method, err := mcpAPIMethod(input.Method, false)
	if err != nil {
		return nil, mcpAPIOutput{}, err
	}
	output, err := adapter.callAPI(ctx, method, input.Path, input.Body, WriteAccess)
	return nil, output, err
}

func mcpAPIMethod(value string, manage bool) (string, error) {
	method := strings.ToUpper(strings.TrimSpace(value))
	if contains([]string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}, method) || manage && method == http.MethodGet {
		return method, nil
	}
	return "", errors.New("method is not available through this MCP tool")
}

func (adapter *Gateway) callAPI(ctx context.Context, method, path string, body any, access AccessClass) (mcpAPIOutput, error) {
	profile, err := adapter.apiPrincipal(ctx)
	if err != nil {
		return mcpAPIOutput{}, err
	}
	parsed, err := mcpAPIPath(path)
	if err != nil {
		return mcpAPIOutput{}, err
	}
	encoded, err := mcpAPIBody(body)
	if err != nil {
		return mcpAPIOutput{}, err
	}
	request, err := http.NewRequestWithContext(ctx, method, parsed.RequestURI(), encoded)
	if err != nil {
		return mcpAPIOutput{}, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	pattern := adapter.api.Pattern(request)
	if pattern == "" || !adapter.routes.Allows(pattern, access) {
		return mcpAPIOutput{}, errors.New("API operation is not available through MCP")
	}
	response := &mcpAPIRecorder{header: make(http.Header), body: mcpBoundedBuffer{limit: mcpAPIResponseLimit}}
	if err := adapter.api.Invoke(response, request, profile, mcpAPIActor(ctx, profile.Name), access == ManageAccess); err != nil {
		return mcpAPIOutput{}, err
	}
	return mcpAPIResponse(pattern, response)
}

func (adapter *Gateway) apiPrincipal(ctx context.Context) (Principal, error) {
	profile, found := stdioPrincipal(ctx)
	if !found {
		token := mcpauth.TokenInfoFromContext(ctx)
		if token != nil {
			profile, found = adapter.principals.ByID(token.UserID)
		}
	}
	if !found {
		return Principal{}, errors.New("authenticated Viewer Profile is unavailable")
	}
	return profile, nil
}

func mcpAPIPath(path string) (*url.URL, error) {
	if len(path) > mcpAPIPathLimit {
		return nil, errors.New("API path exceeds 2048 bytes")
	}
	parsed, err := url.ParseRequestURI(path)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.Path != "/api/v1" && !strings.HasPrefix(parsed.Path, "/api/v1/") {
		return nil, errors.New("path must be a relative /api/v1 URL")
	}
	return parsed, nil
}

func mcpAPIBody(body any) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	data := &mcpBoundedBuffer{limit: mcpAPIBodyLimit}
	if err := json.NewEncoder(data).Encode(body); err != nil {
		return nil, err
	}
	if data.exceeded {
		return nil, errors.New("API request body exceeds 1 MiB")
	}
	return bytes.NewReader(data.Bytes()), nil
}

func mcpAPIActor(ctx context.Context, profileName string) string {
	actor := "MCP · " + profileName
	if local, _ := ctx.Value(stdioClientContextKey{}).(bool); local {
		actor = "Host MCP · " + profileName
	}
	return actor
}

func mcpAPIResponse(pattern string, response *mcpAPIRecorder) (mcpAPIOutput, error) {
	if response.body.exceeded {
		return mcpAPIOutput{}, errors.New("API response exceeds 4 MiB")
	}
	output := mcpAPIOutput{Status: response.statusCode()}
	if response.body.Len() == 0 {
		return output, nil
	}
	mediaType, _, _ := mime.ParseMediaType(response.header.Get("Content-Type"))
	if pattern == "GET /api/v1/metrics" && mediaType == "text/plain" {
		output.Body = response.body.String()
		return output, nil
	}
	if mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json") {
		return mcpAPIOutput{}, errors.New("API response is not JSON and cannot be returned through MCP")
	}
	if err := json.Unmarshal(response.body.Bytes(), &output.Body); err != nil {
		return mcpAPIOutput{}, err
	}
	return output, nil
}
