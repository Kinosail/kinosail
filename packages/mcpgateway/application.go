package mcpgateway

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/auditjournal"
	"github.com/MikeO7/kinosail/packages/federation"
)

// PrincipalAdapterConfig contains one application's private profile operations.
type PrincipalAdapterConfig[P any] struct {
	CurrentProfile       func(*http.Request) P
	FindProfile          func(string) (P, bool)
	FederatedProfiles    func() federation.Profiles[P]
	AllowProfile         func(P, *http.Request, time.Time) bool
	RecentAuthentication func(*http.Request, time.Duration) bool
	Profiles             func() []P
	StateError           func() error
	AttributeProfile     func(*http.Request, P)
	ConvertProfile       func(P) Principal
}

type principalAdapter[P any] struct{ PrincipalAdapterConfig[P] }

// NewPrincipalAdapter validates and builds the identity boundary used by MCP transports.
func NewPrincipalAdapter[P any](config PrincipalAdapterConfig[P]) PrincipalRepository {
	if config.CurrentProfile == nil || config.FindProfile == nil || config.FederatedProfiles == nil || config.AllowProfile == nil || config.RecentAuthentication == nil || config.Profiles == nil || config.StateError == nil || config.AttributeProfile == nil || config.ConvertProfile == nil {
		return nil
	}
	return principalAdapter[P]{config}
}

func (adapter principalAdapter[P]) Current(request *http.Request) Principal {
	return adapter.ConvertProfile(adapter.CurrentProfile(request))
}

func (adapter principalAdapter[P]) ByID(id string) (Principal, bool) {
	profile, found := adapter.FindProfile(id)
	return adapter.ConvertProfile(profile), found
}

func (adapter principalAdapter[P]) ByOIDC(issuer, subject string) (Principal, bool) {
	profile, found := adapter.FederatedProfiles().Find(federation.OIDCProtocol, federation.Identity{Issuer: issuer, Subject: subject})
	return adapter.ConvertProfile(profile), found
}

func (adapter principalAdapter[P]) Allowed(identity Principal, request *http.Request, now time.Time) bool {
	profile, found := adapter.FindProfile(identity.ID)
	return found && adapter.AllowProfile(profile, request, now)
}

func (adapter principalAdapter[P]) RecentlyAuthenticated(request *http.Request, age time.Duration) bool {
	return adapter.RecentAuthentication(request, age)
}

func (adapter principalAdapter[P]) Owners() ([]Principal, error) {
	if err := adapter.StateError(); err != nil {
		return nil, err
	}
	owners := make([]Principal, 0)
	for _, profile := range adapter.Profiles() {
		if identity := adapter.ConvertProfile(profile); identity.Owner {
			owners = append(owners, identity)
		}
	}
	return owners, nil
}

func (adapter principalAdapter[P]) Attribute(request *http.Request, identity Principal) {
	if profile, found := adapter.FindProfile(identity.ID); found {
		adapter.AttributeProfile(request, profile)
	}
}

type apiAdapter[P any, ViewerKey, OwnerKey comparable] struct {
	router       *http.ServeMux
	handler      http.Handler
	find         func(string) (P, bool)
	viewerKey    ViewerKey
	ownerKey     OwnerKey
	serveAudited func(http.Handler, http.ResponseWriter, *http.Request)
}

// NewAPIAdapter validates and builds one application's routed and audited API boundary.
func NewAPIAdapter[P any, ViewerKey, OwnerKey comparable](router *http.ServeMux, handler http.Handler, find func(string) (P, bool), viewerKey ViewerKey, ownerKey OwnerKey, serveAudited func(http.Handler, http.ResponseWriter, *http.Request)) APIInvoker { //nolint:lll // Keeping the typed callbacks together makes the private application boundary explicit.
	if router == nil || handler == nil || find == nil || serveAudited == nil {
		return nil
	}
	return apiAdapter[P, ViewerKey, OwnerKey]{router, handler, find, viewerKey, ownerKey, serveAudited}
}

func (adapter apiAdapter[P, ViewerKey, OwnerKey]) Pattern(request *http.Request) string {
	_, pattern := adapter.router.Handler(request)
	return pattern
}

func (adapter apiAdapter[P, ViewerKey, OwnerKey]) Invoke(writer http.ResponseWriter, request *http.Request, identity Principal, actor string, manage bool) error {
	profile, found := adapter.find(identity.ID)
	if !found {
		return errors.New("authenticated Viewer Profile is unavailable")
	}
	ctx := auditjournal.WithActor(context.WithValue(request.Context(), adapter.viewerKey, profile), actor)
	if manage {
		ctx = context.WithValue(ctx, adapter.ownerKey, true)
	}
	adapter.serveAudited(adapter.handler, writer, request.WithContext(ctx))
	return nil
}

// RegisterApplication installs one application gateway and reports invalid startup configuration.
func RegisterApplication(mux *http.ServeMux, config GatewayConfig, connections *Connections) *Gateway {
	gateway, err := Register(mux, config, connections)
	if err != nil {
		slog.Error("MCP gateway unavailable", "error", err)
	}
	return gateway
}
