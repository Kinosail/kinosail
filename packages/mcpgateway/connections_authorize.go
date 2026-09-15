package mcpgateway

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

func (connections *Connections) authorize(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodPost {
		connections.approve(writer, request)
		return
	}
	profile := connections.principals.Current(request)
	if profile.ID == "" {
		connections.error(writer, request, errors.New("authentication required"), http.StatusUnauthorized)
		return
	}
	transaction, err := connections.authorizationRequest(request, profile)
	if err != nil {
		oauthError(writer, "invalid_request", http.StatusBadRequest)
		return
	}
	requestID := rand.Text()
	connections.mu.Lock()
	connections.pruneLocked()
	if len(connections.pending) >= mcpConnectionLimit {
		connections.mu.Unlock()
		oauthError(writer, "temporarily_unavailable", http.StatusTooManyRequests)
		return
	}
	connections.pending[connections.sessionKey(requestID)] = transaction
	connections.mu.Unlock()
	_ = connections.approval(writer, request, Approval{
		ClientName: transaction.Client.Name, ProfileName: profile.Name, RequestID: requestID,
		Write: slices.Contains(transaction.Scopes, WriteScope), Manage: slices.Contains(transaction.Scopes, ManageScope) && profile.Owner,
	})
}

func (connections *Connections) authorizationRequest(request *http.Request, profile Principal) (mcpOAuthRequest, error) { //nolint:cyclop // OAuth parameters are validated together before pending state is created.
	query := request.URL.Query()
	if !onlyFormKeys(query, "response_type", "client_id", "redirect_uri", "code_challenge", "code_challenge_method", "resource", "state", "scope") {
		return mcpOAuthRequest{}, errors.New("invalid authorization request")
	}
	responseType, ok := oneValue(query, "response_type", 16)
	clientID, clientOK := oneValue(query, "client_id", 2048)
	redirectURI, redirectOK := oneValue(query, "redirect_uri", 2048)
	challenge, challengeOK := oneValue(query, "code_challenge", 128)
	method, methodOK := oneValue(query, "code_challenge_method", 8)
	resource, resourceOK := oneValue(query, "resource", 2048)
	state, stateOK := optionalValue(query, "state", 512)
	scope, scopeOK := optionalValue(query, "scope", 256)
	if !ok || !clientOK || !redirectOK || !challengeOK || !methodOK || !resourceOK || !stateOK || !scopeOK || responseType != "code" || method != "S256" || resource != connections.resource || !validPKCEValue(challenge) {
		return mcpOAuthRequest{}, errors.New("invalid authorization request")
	}
	client, err := connections.clientFor(request.Context(), clientID)
	if err != nil || !redirectAllowed(client.RedirectURIs, redirectURI) {
		return mcpOAuthRequest{}, errors.New("invalid client")
	}
	scopes, err := normalizeMCPScopes(scope, true)
	if err != nil {
		return mcpOAuthRequest{}, errors.New("invalid scope")
	}
	return mcpOAuthRequest{Client: client, RedirectURI: redirectURI, State: state, Challenge: challenge, Scopes: scopes, ProfileID: profile.ID, Expires: connections.now().Add(mcpRequestLifetime).Unix()}, nil
}

func oneValue(values url.Values, key string, limit int) (string, bool) {
	return firstBounded(values[key], limit, true)
}

func optionalValue(values url.Values, key string, limit int) (string, bool) {
	value, found := values[key]
	if !found {
		return "", true
	}
	return firstBounded(value, limit, false)
}

func firstBounded(values []string, limit int, required bool) (string, bool) {
	value := ""
	if len(values) == 1 {
		value = values[0]
	}
	return value, len(values) == 1 && len(value) <= limit && (!required || value != "")
}

func normalizeMCPScopes(raw string, defaultRead bool) ([]string, error) {
	if raw == "" && defaultRead {
		raw = ReadScope
	}
	values := strings.Fields(raw)
	if len(values) == 0 || len(values) > 3 || !allUnique(values) || !every(values, func(value string) bool {
		return value == ReadScope || value == WriteScope || value == ManageScope
	}) || !slices.Contains(values, ReadScope) {
		return nil, errors.New("invalid scope")
	}
	return values, nil
}

func validPKCEValue(value string) bool {
	if len(value) < 43 || len(value) > 128 {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil
}

func (connections *Connections) clientFor(ctx context.Context, id string) (mcpOAuthClient, error) {
	connections.mu.Lock()
	client, found := connections.clients[id]
	connections.mu.Unlock()
	if found {
		return client, nil
	}
	return connections.metadataClient(ctx, id)
}

func (connections *Connections) metadataClient(ctx context.Context, id string) (mcpOAuthClient, error) {
	if !validClientMetadataURL(id) {
		return mcpOAuthClient{}, errors.New("invalid client metadata URL")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, id, nil) //nolint:gosec // The exact HTTPS metadata URL is validated above.
	if err != nil {
		return mcpOAuthClient{}, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := connections.client.Do(request) //nolint:gosec // The transport resolves and pins only public, non-special-purpose IP addresses.
	if err != nil {
		return mcpOAuthClient{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return mcpOAuthClient{}, errors.New("client metadata unavailable")
	}
	var metadata mcpClientMetadataDocument
	if httpguard.DecodeJSON(response.Body, 64<<10, &metadata, true) != nil || metadata.ClientID != id || validateMCPClientMetadata(&metadata.ClientRegistrationMetadata) != nil {
		return mcpOAuthClient{}, errors.New("invalid client metadata")
	}
	return mcpOAuthClient{ID: id, Name: metadata.ClientName, RedirectURIs: metadata.RedirectURIs, MetadataURL: id}, nil
}

func validClientMetadataURL(id string) bool {
	if len(id) > 2048 {
		return false
	}
	parsed, err := url.Parse(id)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return false
	}
	return parsed.RawQuery == "" && parsed.Fragment == "" && parsed.Path != "" && parsed.Path != "/"
}

func redirectAllowed(registered []string, actual string) bool {
	if !validMCPRedirect(actual) {
		return false
	}
	for _, expected := range registered {
		if expected == actual || sameLoopbackRedirect(expected, actual) {
			return true
		}
	}
	return false
}

func sameLoopbackRedirect(expected, actual string) bool {
	left, leftErr := url.Parse(expected)
	right, rightErr := url.Parse(actual)
	return leftErr == nil && rightErr == nil && left.Scheme == "http" && right.Scheme == "http" && loopbackHost(left.Hostname()) && left.Hostname() == right.Hostname() && left.Port() == "" && right.Port() != "" && left.Path == right.Path && left.RawQuery == right.RawQuery
}

func (connections *Connections) approve(writer http.ResponseWriter, request *http.Request) {
	requestID, decision, scopesInput, valuesOK, formOK := approvalForm(writer, request)
	if !formOK {
		oauthError(writer, "invalid_request", http.StatusBadRequest)
		return
	}
	pending, found := connections.takePending(requestID)
	profile := connections.principals.Current(request)
	if !valuesOK || !validApproval(decision, pending, found, profile, connections.now()) {
		oauthError(writer, "invalid_request", http.StatusBadRequest)
		return
	}
	if decision == "deny" {
		connections.redirectAuthorization(writer, request, pending, url.Values{"error": {"access_denied"}})
		return
	}
	scopes, allowed := connections.approvedScopes(request, pending, profile, scopesInput)
	if !allowed {
		oauthError(writer, "invalid_scope", http.StatusForbidden)
		return
	}
	pending.Scopes = scopes
	code := rand.Text()
	connections.mu.Lock()
	connections.codes[secretHash(code)] = mcpOAuthCode{mcpOAuthRequest: pending, Expires: connections.now().Add(mcpRequestLifetime).Unix()}
	connections.mu.Unlock()
	connections.redirectAuthorization(writer, request, pending, url.Values{"code": {code}})
}

func approvalForm(writer http.ResponseWriter, request *http.Request) (string, string, []string, bool, bool) {
	request.Body = http.MaxBytesReader(writer, request.Body, 16<<10)
	if !formEncoded(request) || request.URL.RawQuery != "" || request.ParseForm() != nil || !onlyFormKeys(request.PostForm, "request", "decision", "scopes") {
		return "", "", nil, false, false
	}
	requestID, requestOK := oneValue(request.PostForm, "request", 128)
	decision, decisionOK := oneValue(request.PostForm, "decision", 8)
	return requestID, decision, request.PostForm["scopes"], requestOK && decisionOK, true
}

func (connections *Connections) takePending(requestID string) (mcpOAuthRequest, bool) {
	connections.mu.Lock()
	pending, found := connections.pending[connections.sessionKey(requestID)]
	delete(connections.pending, connections.sessionKey(requestID))
	connections.mu.Unlock()
	return pending, found
}

func validApproval(decision string, pending mcpOAuthRequest, found bool, profile Principal, now time.Time) bool {
	return found && now.Unix() <= pending.Expires && profile.ID != "" && profile.ID == pending.ProfileID && (decision == "allow" || decision == "deny")
}

func (connections *Connections) approvedScopes(request *http.Request, pending mcpOAuthRequest, profile Principal, scopesInput []string) ([]string, bool) {
	if len(scopesInput) == 0 || len(scopesInput) > 3 || !allUnique(scopesInput) {
		return nil, false
	}
	scopes, err := normalizeMCPScopes(strings.Join(scopesInput, " "), false)
	if err != nil || !every(scopes, func(scope string) bool { return slices.Contains(pending.Scopes, scope) }) || slices.Contains(scopes, ManageScope) && (!profile.Owner || profile.ID != "local-owner" && !connections.principals.RecentlyAuthenticated(request, 10*time.Minute)) {
		return nil, false
	}
	return scopes, true
}

func (connections *Connections) redirectAuthorization(writer http.ResponseWriter, request *http.Request, pending mcpOAuthRequest, values url.Values) {
	redirect, _ := url.Parse(pending.RedirectURI)
	query := redirect.Query()
	for key, value := range values {
		query[key] = value
	}
	if pending.State != "" {
		query.Set("state", pending.State)
	}
	query.Set("iss", connections.issuer)
	redirect.RawQuery = query.Encode()
	http.Redirect(writer, request, redirect.String(), http.StatusFound) //nolint:gosec // The exact registered redirect URI was validated before pending state was created.
}
