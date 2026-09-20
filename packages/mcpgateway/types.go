package mcpgateway

import (
	"context"
	"errors"
	"net/http"
	"time"
)

const (
	ProtocolVersion = "2026-07-28"
	ReadScope       = "kinosail.read"
	WriteScope      = "kinosail.write"
	ManageScope     = "kinosail.manage"
	StateFilename   = "agent_connections.json"
)

// Principal is the identity data required by the MCP gateway.
type Principal struct {
	Revision uint64
	ID, Name string
	Owner    bool
}

// PrincipalRepository adapts the application identity model to MCP authorization.
type PrincipalRepository interface {
	Current(*http.Request) Principal
	ByID(string) (Principal, bool)
	ByOIDC(issuer, subject string) (Principal, bool)
	Allowed(Principal, *http.Request, time.Time) bool
	RecentlyAuthenticated(*http.Request, time.Duration) bool
	Owners() ([]Principal, error)
	Attribute(*http.Request, Principal)
}

// APIInvoker calls the application API with its native identity and audit contexts.
type APIInvoker interface {
	Pattern(*http.Request) string
	Invoke(http.ResponseWriter, *http.Request, Principal, string, bool) error
}

// AccessClass identifies the authority required for an API route.
type AccessClass uint8

const (
	ReadAccess AccessClass = iota
	WriteAccess
	ManageAccess
)

// RoutePolicy defines the exact application routes available to each access class.
type RoutePolicy interface {
	Allows(pattern string, access AccessClass) bool
}

// StateStore persists the gateway's opaque client and grant state.
type StateStore interface {
	Load(any) (bool, error)
	Save(any) error
}

// Approval describes one validated browser approval request.
type Approval struct {
	ClientName, ProfileName, RequestID string
	Write, Manage                      bool
}

// ApprovalPresenter renders the product-owned approval page.
type ApprovalPresenter func(http.ResponseWriter, *http.Request, Approval) error

// ErrorWriter translates application-facing failures into product responses.
type ErrorWriter func(http.ResponseWriter, *http.Request, error, int)

// GatewayConfig contains application adapters for one MCP gateway.
type GatewayConfig struct {
	OAuth      OAuthConfig
	Principals PrincipalRepository
	API        APIInvoker
	Routes     RoutePolicy
}

func (config GatewayConfig) validate() error {
	if config.Principals == nil || config.API == nil || config.Routes == nil {
		return errors.New("MCP gateway adapters are unavailable")
	}
	return nil
}

type (
	principalContextKey   struct{}
	stdioClientContextKey struct{}
)

func withStdioPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(context.WithValue(ctx, principalContextKey{}, principal), stdioClientContextKey{}, true)
}

func stdioPrincipal(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok && principal.Owner
}
