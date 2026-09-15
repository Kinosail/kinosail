package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/mcpgateway"
)

const (
	mcpProtocolVersion = mcpgateway.ProtocolVersion
	mcpReadScope       = mcpgateway.ReadScope
	mcpWriteScope      = mcpgateway.WriteScope
	mcpManageScope     = mcpgateway.ManageScope
)

type (
	MCPConfig  = mcpgateway.OAuthConfig
	mcpAdapter = mcpgateway.Gateway
)

func mcpProfileState(store *profileStore) mcpgateway.ProfileState {
	return mcpgateway.ProfileState{Mutex: &store.mu, Profiles: &store.profiles, Error: &store.err}
}

func mcpPrincipals(store *profileStore) mcpgateway.PrincipalRepository {
	return mcpgateway.NewProfilePrincipals(mcpgateway.ProfilePrincipalConfig{
		State: mcpProfileState(store), CurrentProfile: currentViewer, RecentlyAuthenticated: store.recentlyAuthenticated,
		AttributeProfile: setAuditViewer, PublicRequest: publicInternetRequest,
	})
}

func mcpAPI(mux *http.ServeMux, api http.Handler, authentication *authentication) mcpgateway.APIInvoker {
	return mcpgateway.NewAPIAdapter(mux, api, authentication.profiles.byID, viewerContextKey{}, ownerAutomationContextKey{}, authentication.audit.Track)
}

func registerMCPWithConnections(mux *http.ServeMux, config MCPConfig, authentication *authentication, api http.Handler, connections *mcpConnections) *mcpAdapter {
	var builtIn *mcpgateway.Connections
	if connections != nil {
		builtIn = connections.Connections
	}
	return mcpgateway.RegisterApplication(mux, mcpgateway.GatewayConfig{
		OAuth: config, Principals: mcpPrincipals(authentication.profiles), API: mcpAPI(mux, api, authentication), Routes: mcpRoutePolicy{},
	}, builtIn)
}

func mcpError(writer http.ResponseWriter, request *http.Request, err error, status int) {
	if request == nil {
		apiError(writer, err, status)
		return
	}
	localizedError(writer, request, err.Error(), status)
}
