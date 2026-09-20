package mcpgateway

import (
	"crypto/rand"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

func (connections *Connections) registerClient(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop // Registration validation is one protocol boundary.
	if connections.err != nil {
		oauthError(writer, "temporarily_unavailable", http.StatusServiceUnavailable)
		return
	}
	if !connections.register.Allow("mcp:"+httpguard.RemoteHost(request.RemoteAddr), 20) {
		writer.Header().Set("Retry-After", "60")
		oauthError(writer, "temporarily_unavailable", http.StatusTooManyRequests)
		return
	}
	if mediaType, _, _ := mime.ParseMediaType(request.Header.Get("Content-Type")); mediaType != "application/json" || request.URL.RawQuery != "" {
		oauthError(writer, "invalid_client_metadata", http.StatusBadRequest)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 64<<10)
	var metadata oauthex.ClientRegistrationMetadata
	if httpguard.DecodeUniqueJSON(request.Body, 64<<10, &metadata) != nil || validateMCPClientMetadata(&metadata) != nil {
		oauthError(writer, "invalid_client_metadata", http.StatusBadRequest)
		return
	}
	client := mcpOAuthClient{ID: "kinosail_" + rand.Text(), Name: strings.TrimSpace(metadata.ClientName), RedirectURIs: append([]string(nil), metadata.RedirectURIs...), CreatedAt: connections.now().Unix()}
	connections.mu.Lock()
	defer connections.mu.Unlock()
	connections.pruneLocked()
	if len(connections.clients) >= mcpConnectionLimit {
		oauthError(writer, "invalid_client_metadata", http.StatusTooManyRequests)
		return
	}
	state := connections.stateLocked()
	state.Clients[client.ID] = client
	if connections.commitLocked(state) != nil {
		oauthError(writer, "temporarily_unavailable", http.StatusServiceUnavailable)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, &oauthex.ClientRegistrationResponse{ClientRegistrationMetadata: metadata, ClientID: client.ID, ClientIDIssuedAt: time.Unix(client.CreatedAt, 0)}, http.StatusCreated)
}

func validateMCPClientMetadata(metadata *oauthex.ClientRegistrationMetadata) error { //nolint:cyclop // OAuth metadata fields are validated as one closed contract.
	metadata.ClientName = strings.TrimSpace(metadata.ClientName)
	if !validMCPClientMetadataSizes(metadata) {
		return errors.New("invalid client metadata")
	}
	if metadata.TokenEndpointAuthMethod == "" {
		metadata.TokenEndpointAuthMethod = "none"
	}
	if metadata.TokenEndpointAuthMethod != "none" || !allowedValues(metadata.GrantTypes, []string{"authorization_code", "refresh_token"}, "authorization_code") || !allowedValues(metadata.ResponseTypes, []string{"code"}, "code") {
		return errors.New("invalid client metadata")
	}
	if !validMCPClientMetadataFields(metadata) {
		return errors.New("invalid client metadata")
	}
	if !every(metadata.Contacts, func(contact string) bool { return contact != "" && len(contact) <= 254 }) {
		return errors.New("invalid client metadata")
	}
	for _, redirect := range metadata.RedirectURIs {
		if !validMCPRedirect(redirect) {
			return errors.New("invalid redirect URI")
		}
	}
	if metadata.Scope != "" {
		_, err := normalizeMCPScopes(metadata.Scope, false)
		return err
	}
	return nil
}

func validMCPClientMetadataSizes(metadata *oauthex.ClientRegistrationMetadata) bool {
	return metadata.ClientName != "" && len(metadata.ClientName) <= 80 && len(metadata.RedirectURIs) > 0 && len(metadata.RedirectURIs) <= 8 && len(metadata.Scope) <= 256 && len(metadata.Contacts) <= 8
}

func validMCPClientMetadataFields(metadata *oauthex.ClientRegistrationMetadata) bool {
	applicationType := metadata.ApplicationType == "" || metadata.ApplicationType == "native" || metadata.ApplicationType == "web"
	return applicationType && len(metadata.ClientURI) <= 2048 && len(metadata.LogoURI) <= 2048 && len(metadata.TOSURI) <= 2048 && len(metadata.PolicyURI) <= 2048 && len(metadata.JWKSURI) <= 2048 && len(metadata.SoftwareID) <= 128 && len(metadata.SoftwareVersion) <= 128
}

func allowedValues(values, allowed []string, defaultValue string) bool {
	if len(values) == 0 {
		values = []string{defaultValue}
	}
	return len(values) <= len(allowed) && allUnique(values) && every(values, func(value string) bool { return slices.Contains(allowed, value) })
}

func allUnique(values []string) bool {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}

func every(values []string, valid func(string) bool) bool {
	for _, value := range values {
		if !valid(value) {
			return false
		}
	}
	return true
}

func validMCPRedirect(raw string) bool {
	if raw == "" || len(raw) > 2048 || strings.Contains(raw, `\`) {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	return parsed.Scheme == "https" || parsed.Scheme == "http" && loopbackHost(parsed.Hostname())
}
