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

type Principal struct {
	Revision uint64
	ID, Name string
	Owner    bool
}

type PrincipalRepository interface {
	Current(*http.Request) Principal
	ByID(string) (Principal, bool)
	ByOIDC(issuer, subject string) (Principal, bool)
	Allowed(Principal, *http.Request, time.Time) bool
	RecentlyAuthenticated(*http.Request, time.Duration) bool
	Owners() ([]Principal, error)
	Attribute(*http.Request, Principal)
}

type APIInvoker interface {
	Pattern(*http.Request) string
	Invoke(http.ResponseWriter, *http.Request, Principal, string, bool) error
}

type AccessClass uint8

const (
	ReadAccess AccessClass = iota
	WriteAccess
	ManageAccess
)

type RoutePolicy interface {
	Allows(pattern string, access AccessClass) bool
}

type StateStore interface {
	Load(any) (bool, error)
	Save(any) error
}

type Approval struct {
	ClientName, ProfileName, RequestID string
	Write, Manage                      bool
}

type ApprovalPresenter func(http.ResponseWriter, *http.Request, Approval) error

type ErrorWriter func(http.ResponseWriter, *http.Request, error, int)

type GatewayConfig struct {
	OAuth          OAuthConfig
	Principals     PrincipalRepository
	API            APIInvoker
	Routes         RoutePolicy
	SubtitleEvents bool
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
