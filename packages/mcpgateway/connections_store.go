package mcpgateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

// Views returns the safe active-grant projection.
func (connections *Connections) Views() []ConnectionView {
	connections.mu.Lock()
	defer connections.mu.Unlock()
	connections.pruneLocked()
	views := make([]mcpConnectionView, 0, len(connections.grants))
	for _, grant := range connections.grants {
		if connections.now().Unix() > grant.RefreshExpires {
			continue
		}
		profile, _ := connections.principals.ByID(grant.ProfileID)
		view := mcpConnectionView{ID: grant.ID, ClientName: grant.ClientName, ProfileName: profile.Name, Created: formatConnectionTime(grant.CreatedAt), Expires: formatConnectionTime(grant.RefreshExpires), Scopes: append([]string(nil), grant.Scopes...)}
		if grant.LastUsed != 0 {
			view.LastUsed = formatConnectionTime(grant.LastUsed)
		}
		views = append(views, view)
	}
	slices.SortFunc(views, func(left, right mcpConnectionView) int {
		return strings.Compare(left.ClientName+left.ID, right.ClientName+right.ID)
	})
	return views
}

func formatConnectionTime(value int64) string {
	return time.Unix(value, 0).Local().Format(time.RFC3339)
}

// Revoke removes one grant by its opaque identifier.
func (connections *Connections) Revoke(id string) error {
	if id == "" || len(id) > 128 {
		return errors.New("connection was not found")
	}
	connections.mu.Lock()
	defer connections.mu.Unlock()
	if _, found := connections.grants[id]; !found {
		return errors.New("connection was not found")
	}
	state := connections.stateLocked()
	delete(state.Grants, id)
	return connections.commitLocked(state)
}

func (connections *Connections) stateLocked() mcpConnectionState {
	state := mcpConnectionState{Clients: make(map[string]mcpOAuthClient, len(connections.clients)), Grants: make(map[string]mcpOAuthGrant, len(connections.grants))}
	for id, client := range connections.clients {
		client.RedirectURIs = append([]string(nil), client.RedirectURIs...)
		state.Clients[id] = client
	}
	for id, grant := range connections.grants {
		grant.Scopes = append([]string(nil), grant.Scopes...)
		state.Grants[id] = grant
	}
	return state
}

func (connections *Connections) commitLocked(state mcpConnectionState) error {
	if err := connections.store.Save(state); err != nil {
		return err
	}
	connections.clients, connections.grants = state.Clients, state.Grants
	return nil
}

func (connections *Connections) pruneLocked() {
	now := connections.now().Unix()
	for id, pending := range connections.pending {
		if now > pending.Expires {
			delete(connections.pending, id)
		}
	}
	for id, code := range connections.codes {
		if now > code.Expires {
			delete(connections.codes, id)
		}
	}
}

func secretHash(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func oauthError(writer http.ResponseWriter, code string, status int) {
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, map[string]string{"error": code}, status)
}

func publicMetadataHTTPClient(timeout time.Duration) *http.Client { //nolint:cyclop // DNS resolution and every prohibited network range fail closed in one dial boundary.
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, errors.New("outbound address is invalid")
		}
		addresses, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, errors.New("outbound host could not be resolved")
		}
		for _, resolved := range addresses {
			if identitycore.AllowedOutboundIP(resolved) {
				return dialer.DialContext(ctx, network, net.JoinHostPort(resolved.String(), port))
			}
		}
		return nil, errors.New("outbound host resolved only to prohibited addresses")
	}
	return &http.Client{Timeout: timeout, Transport: transport, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}
