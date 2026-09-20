package mcpgateway

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

const (
	mcpAccessLifetime  = time.Hour
	mcpRefreshLifetime = 30 * 24 * time.Hour
	mcpRequestLifetime = 10 * time.Minute
	mcpConnectionLimit = 128
)

// Connections owns built-in OAuth clients, grants, and one-use credentials.
type Connections struct {
	mu         sync.Mutex
	issuer     string
	resource   string
	principals PrincipalRepository
	store      StateStore
	error      ErrorWriter
	approval   ApprovalPresenter
	sessionKey func(string) string
	clients    map[string]mcpOAuthClient
	grants     map[string]mcpOAuthGrant
	pending    map[string]mcpOAuthRequest
	codes      map[string]mcpOAuthCode
	err        error
	now        func() time.Time
	client     *http.Client
	external   bool
	register   httpguard.Limiter
}

type mcpConnectionState struct {
	Clients map[string]mcpOAuthClient `json:"clients"`
	Grants  map[string]mcpOAuthGrant  `json:"grants"`
}

type mcpOAuthClient struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	RedirectURIs []string `json:"redirectUris"`
	CreatedAt    int64    `json:"createdAt"`
	MetadataURL  string   `json:"metadataUrl,omitempty"`
}

type mcpOAuthGrant struct {
	ProfileRevision uint64   `json:"profileRevision"`
	ID              string   `json:"id"`
	ClientID        string   `json:"clientId"`
	ClientName      string   `json:"clientName"`
	ProfileID       string   `json:"profileId"`
	Scopes          []string `json:"scopes"`
	CreatedAt       int64    `json:"createdAt"`
	LastUsed        int64    `json:"lastUsed,omitempty"`
	AccessHash      string   `json:"accessHash"`
	AccessExpires   int64    `json:"accessExpires"`
	RefreshHash     string   `json:"refreshHash"`
	RefreshExpires  int64    `json:"refreshExpires"`
}

type mcpOAuthRequest struct {
	ProfileRevision uint64
	Client          mcpOAuthClient
	RedirectURI     string
	State           string
	Challenge       string
	Scopes          []string
	ProfileID       string
	Expires         int64
}

type mcpOAuthCode struct {
	mcpOAuthRequest
	Expires int64
}

type mcpConnectionView struct {
	ID          string   `json:"id"`
	ClientName  string   `json:"clientName"`
	ProfileName string   `json:"profileName"`
	Created     string   `json:"created"`
	LastUsed    string   `json:"lastUsed,omitempty"`
	Expires     string   `json:"expires"`
	Scopes      []string `json:"scopes"`
}

// ConnectionView is one safe, revocable grant projection.
type ConnectionView = mcpConnectionView

type mcpClientMetadataDocument struct {
	ClientID string `json:"client_id"`
	oauthex.ClientRegistrationMetadata
}

// ConnectionConfig contains the adapters required by built-in OAuth.
type ConnectionConfig struct {
	Issuer, Resource string
	Principals       PrincipalRepository
	Store            StateStore
	Error            ErrorWriter
	Approval         ApprovalPresenter
	SessionKey       func(string) string
}

// NewConnections loads built-in OAuth state without creating authority on failure.
func NewConnections(config ConnectionConfig) *Connections { //nolint:contextcheck // Startup state loading is installation-owned.
	issuer := strings.TrimSuffix(strings.TrimSpace(config.Issuer), "/")
	if issuer == "" {
		issuer = "http://localhost"
	}
	resource := strings.TrimSpace(config.Resource)
	if resource == "" {
		resource = issuer + "/mcp"
	}
	connections := &Connections{
		issuer: issuer, resource: resource, principals: config.Principals, store: config.Store, error: config.Error, approval: config.Approval, sessionKey: config.SessionKey,
		clients: make(map[string]mcpOAuthClient), grants: make(map[string]mcpOAuthGrant),
		pending: make(map[string]mcpOAuthRequest), codes: make(map[string]mcpOAuthCode), now: time.Now, client: publicMetadataHTTPClient(10 * time.Second),
	}
	if config.Principals == nil || config.Store == nil || config.Error == nil || config.Approval == nil || config.SessionKey == nil || !validMCPIssuer(issuer) {
		connections.err = errors.New("agent connections are unavailable")
		return connections
	}
	connections.loadState()
	return connections
}

func (connections *Connections) loadState() {
	var state mcpConnectionState
	found, err := connections.store.Load(&state)
	if err != nil {
		connections.err = err
		return
	}
	if !found {
		return
	}
	if state.Clients != nil {
		connections.clients = state.Clients
	}
	if state.Grants != nil {
		connections.grants = state.Grants
	}
}

// Configure selects an external OAuth provider when configured.
func (connections *Connections) Configure(config OAuthConfig) {
	if config.Configured() {
		connections.external, connections.issuer, connections.resource = true, config.AuthorizationServer, config.ResourceURL
	}
}

// RegisterOAuth installs the built-in authorization server routes.
func (connections *Connections) RegisterOAuth(mux *http.ServeMux) {
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", connections.metadata)
	mux.HandleFunc("POST /oauth/register", connections.registerClient)
	mux.HandleFunc("GET /oauth/authorize", connections.authorize)
	mux.HandleFunc("POST /oauth/authorize", connections.authorize)
	mux.HandleFunc("POST /oauth/token", connections.token)
	mux.HandleFunc("POST /oauth/revoke", connections.revokeToken)
}

func (connections *Connections) metadata(writer http.ResponseWriter, _ *http.Request) {
	if connections.err != nil || !validMCPIssuer(connections.issuer) {
		connections.error(writer, nil, errors.New("agent connections are unavailable"), http.StatusServiceUnavailable)
		return
	}
	writeJSON(writer, oauthex.AuthServerMeta{
		Issuer: connections.issuer, AuthorizationEndpoint: connections.issuer + "/oauth/authorize", TokenEndpoint: connections.issuer + "/oauth/token",
		RegistrationEndpoint: connections.issuer + "/oauth/register", RevocationEndpoint: connections.issuer + "/oauth/revoke",
		ScopesSupported: []string{ReadScope, WriteScope, ManageScope}, ResponseTypesSupported: []string{"code"},
		GrantTypesSupported: []string{"authorization_code", "refresh_token"}, TokenEndpointAuthMethodsSupported: []string{"none"},
		RevocationEndpointAuthMethodsSupported: []string{"none"}, CodeChallengeMethodsSupported: []string{"S256"},
		ClientIDMetadataDocumentSupported: true, AuthorizationResponseIssParameterSupported: true,
	}, http.StatusOK)
}

func validMCPIssuer(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && trustedURL(parsed) && (parsed.Path == "" || parsed.Path == "/")
}

// Issuer returns the active authorization server.
func (connections *Connections) Issuer() string { return connections.issuer }

// Resource returns the protected resource address.
func (connections *Connections) Resource() string { return connections.resource }

// External reports whether an external OAuth provider is active.
func (connections *Connections) External() bool { return connections.external }
