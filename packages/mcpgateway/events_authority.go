package mcpgateway

import (
	"context"
	"net/http"

	"github.com/MikeO7/kinosail/packages/identitycore"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

func (adapter *Gateway) eventManage(ctx context.Context) bool {
	if _, found := stdioPrincipal(ctx); found {
		return true
	}
	token := mcpauth.TokenInfoFromContext(ctx)
	return token != nil && contains(token.Scopes, ManageScope)
}

func (adapter *Gateway) eventIdentity(ctx context.Context, name string) (eventSubscription, error) { //nolint:cyclop // Host, grant, and external authority require distinct fail-closed checks.
	profile, err := adapter.apiPrincipal(ctx)
	if err != nil {
		return eventSubscription{}, eventError(-32012, "event access is unavailable", nil)
	}
	_, _, management := eventKind(name)
	if management && (!adapter.subtitleEvents || !profile.Owner || !adapter.eventManage(ctx)) {
		return eventSubscription{}, eventError(-32012, "Owner management access is required", nil)
	}
	identity := eventSubscription{Principal: profile, Scope: "host:" + profile.ID, Name: name}
	if _, found := stdioPrincipal(ctx); found {
		return identity, nil
	}
	token := mcpauth.TokenInfoFromContext(ctx)
	if token == nil || !contains(token.Scopes, ReadScope) {
		return eventSubscription{}, eventError(-32012, "event access is unavailable", nil)
	}
	identity.Grant, _ = token.Extra["eventGrant"].(string)
	identity.Remote, _ = token.Extra["eventRemote"].(bool)
	if identity.Grant != "" {
		identity.Scope = "grant:" + identity.Grant + ":" + profile.ID
		return identity, nil
	}
	client, _ := token.Extra["eventClient"].(string)
	if client == "" || len(client) > 2048 {
		return eventSubscription{}, eventError(-32014, "external OAuth must identify the client for event subscriptions", map[string]string{"feature": "clientIdentity"})
	}
	identity.Scope = "external:" + client + ":" + profile.ID
	identity.AuthorityExpires = token.Expiration.Unix()
	return identity, nil
}

func (adapter *Gateway) eventAllowed(ctx context.Context, subscription eventSubscription) bool { //nolint:cyclop,gocognit // Current profile, grant, and network policy must all authorize each delivery.
	now := adapter.connections.now()
	if subscription.Expires <= now.Unix() || subscription.AuthorityExpires != 0 && subscription.AuthorityExpires <= now.Unix() {
		return false
	}
	profile, found := adapter.principals.ByID(subscription.Principal.ID)
	if !found || profile.Revision != subscription.Principal.Revision {
		return false
	}
	if subscription.Scope == "host:"+profile.ID && !profile.Owner {
		return false
	}
	_, _, management := eventKind(subscription.Name)
	if management && (!adapter.subtitleEvents || !profile.Owner) {
		return false
	}
	if subscription.Grant != "" {
		adapter.connections.mu.Lock()
		grant, exists := adapter.connections.grants[subscription.Grant]
		adapter.connections.mu.Unlock()
		if !exists || grant.ProfileID != profile.ID || grant.ProfileRevision != profile.Revision || grant.RefreshExpires <= now.Unix() || !contains(grant.Scopes, ReadScope) || management && !contains(grant.Scopes, ManageScope) {
			return false
		}
	}
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, adapter.connections.resource, nil)
	if subscription.Remote {
		identitycore.Remote(http.HandlerFunc(func(_ http.ResponseWriter, remote *http.Request) { request = remote })).ServeHTTP(nil, request)
	}
	return adapter.principals.Allowed(profile, request, now)
}
