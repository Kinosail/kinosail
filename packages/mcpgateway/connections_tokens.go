package mcpgateway

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

func (connections *Connections) token(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop // Grant-specific validation stays ordered and consumes one-use credentials.
	if !formEncoded(request) || request.Header.Get("Authorization") != "" || request.URL.RawQuery != "" || request.ContentLength > 64<<10 {
		oauthError(writer, "invalid_request", http.StatusBadRequest)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 64<<10)
	if request.ParseForm() != nil {
		oauthError(writer, "invalid_request", http.StatusBadRequest)
		return
	}
	grantType, grantOK := oneValue(request.PostForm, "grant_type", 32)
	clientID, clientOK := oneValue(request.PostForm, "client_id", 2048)
	resource, resourceOK := oneValue(request.PostForm, "resource", 2048)
	if !grantOK || !clientOK || !resourceOK || resource != connections.resource {
		oauthError(writer, "invalid_request", http.StatusBadRequest)
		return
	}
	switch grantType {
	case "authorization_code":
		connections.exchangeCode(writer, request, clientID)
	case "refresh_token":
		connections.refresh(writer, request, clientID)
	default:
		oauthError(writer, "unsupported_grant_type", http.StatusBadRequest)
	}
}

func (connections *Connections) exchangeCode(writer http.ResponseWriter, request *http.Request, clientID string) { //nolint:cyclop // The one-use authorization grant is validated and consumed as one boundary.
	form := request.PostForm
	code, codeOK := oneValue(form, "code", 128)
	redirectURI, redirectOK := oneValue(form, "redirect_uri", 2048)
	verifier, verifierOK := oneValue(form, "code_verifier", 128)
	if !codeOK || !redirectOK || !verifierOK || !validPKCEValue(verifier) || !onlyFormKeys(form, "grant_type", "client_id", "resource", "code", "redirect_uri", "code_verifier") {
		oauthError(writer, "invalid_grant", http.StatusBadRequest)
		return
	}
	connections.mu.Lock()
	transaction, found := connections.codes[secretHash(code)]
	delete(connections.codes, secretHash(code))
	connections.mu.Unlock()
	digest := sha256.Sum256([]byte(verifier))
	if !found || connections.now().Unix() > transaction.Expires || transaction.Client.ID != clientID || transaction.RedirectURI != redirectURI || base64.RawURLEncoding.EncodeToString(digest[:]) != transaction.Challenge {
		oauthError(writer, "invalid_grant", http.StatusBadRequest)
		return
	}
	profile, found := connections.principals.ByID(transaction.ProfileID)
	if !found || !connections.principals.Allowed(profile, request, connections.now()) {
		oauthError(writer, "invalid_grant", http.StatusBadRequest)
		return
	}
	connections.issue(writer, transaction.Client, transaction.ProfileID, transaction.Scopes)
}

func (connections *Connections) refresh(writer http.ResponseWriter, request *http.Request, clientID string) { //nolint:cyclop // Refresh lookup, profile authorization, and rotation are one atomic operation.
	form := request.PostForm
	refresh, ok := oneValue(form, "refresh_token", 128)
	if !ok || !onlyFormKeys(form, "grant_type", "client_id", "resource", "refresh_token") {
		oauthError(writer, "invalid_grant", http.StatusBadRequest)
		return
	}
	hash := secretHash(refresh)
	connections.mu.Lock()
	defer connections.mu.Unlock()
	var grant mcpOAuthGrant
	for _, candidate := range connections.grants {
		if candidate.RefreshHash == hash {
			grant = candidate
			break
		}
	}
	if grant.ID == "" || grant.ClientID != clientID || connections.now().Unix() > grant.RefreshExpires {
		oauthError(writer, "invalid_grant", http.StatusBadRequest)
		return
	}
	profile, found := connections.principals.ByID(grant.ProfileID)
	if !found || !connections.principals.Allowed(profile, request, connections.now()) {
		oauthError(writer, "invalid_grant", http.StatusBadRequest)
		return
	}
	access, rotated := rand.Text(), rand.Text()
	now := connections.now()
	grant.AccessHash, grant.AccessExpires = secretHash(access), now.Add(mcpAccessLifetime).Unix()
	grant.RefreshHash, grant.RefreshExpires = secretHash(rotated), now.Add(mcpRefreshLifetime).Unix()
	state := connections.stateLocked()
	state.Grants[grant.ID] = grant
	if connections.commitLocked(state) != nil {
		oauthError(writer, "temporarily_unavailable", http.StatusServiceUnavailable)
		return
	}
	writeMCPToken(writer, access, rotated, grant.Scopes)
}

func onlyFormKeys(form url.Values, keys ...string) bool {
	for key := range form {
		if !slices.Contains(keys, key) {
			return false
		}
	}
	return true
}

func (connections *Connections) issue(writer http.ResponseWriter, client mcpOAuthClient, profileID string, scopes []string) {
	access, refresh := rand.Text(), rand.Text()
	now := connections.now()
	grant := mcpOAuthGrant{ID: rand.Text(), ClientID: client.ID, ClientName: client.Name, ProfileID: profileID, Scopes: append([]string(nil), scopes...), CreatedAt: now.Unix(), AccessHash: secretHash(access), AccessExpires: now.Add(mcpAccessLifetime).Unix(), RefreshHash: secretHash(refresh), RefreshExpires: now.Add(mcpRefreshLifetime).Unix()}
	connections.mu.Lock()
	defer connections.mu.Unlock()
	if len(connections.grants) >= mcpConnectionLimit {
		oauthError(writer, "temporarily_unavailable", http.StatusTooManyRequests)
		return
	}
	state := connections.stateLocked()
	state.Grants[grant.ID] = grant
	if connections.commitLocked(state) != nil {
		oauthError(writer, "temporarily_unavailable", http.StatusServiceUnavailable)
		return
	}
	writeMCPToken(writer, access, refresh, scopes)
}

func writeMCPToken(writer http.ResponseWriter, access, refresh string, scopes []string) {
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, map[string]any{"access_token": access, "token_type": "Bearer", "expires_in": int(mcpAccessLifetime.Seconds()), "refresh_token": refresh, "scope": strings.Join(scopes, " ")}, http.StatusOK)
}

// VerifyToken validates one built-in access token and records its last use.
func (connections *Connections) VerifyToken(_ context.Context, token string, request *http.Request) (*mcpauth.TokenInfo, error) {
	hash, now := secretHash(token), connections.now()
	connections.mu.Lock()
	defer connections.mu.Unlock()
	for id, grant := range connections.grants {
		if grant.AccessHash != hash || now.Unix() > grant.AccessExpires {
			continue
		}
		profile, found := connections.principals.ByID(grant.ProfileID)
		if !found || !connections.principals.Allowed(profile, request, now) {
			return nil, mcpauth.ErrInvalidToken
		}
		grant.LastUsed = now.Unix()
		state := connections.stateLocked()
		state.Grants[id] = grant
		if connections.commitLocked(state) != nil {
			return nil, mcpauth.ErrInvalidToken
		}
		connections.principals.Attribute(request, profile)
		return &mcpauth.TokenInfo{Scopes: append([]string(nil), grant.Scopes...), Expiration: time.Unix(grant.AccessExpires, 0), UserID: grant.ProfileID}, nil
	}
	return nil, mcpauth.ErrInvalidToken
}

func (connections *Connections) revokeToken(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop // Strict revocation validation and token lookup stay together at the protocol boundary.
	request.Body = http.MaxBytesReader(writer, request.Body, 16<<10)
	if !formEncoded(request) || request.Header.Get("Authorization") != "" || request.URL.RawQuery != "" || request.ParseForm() != nil || !onlyFormKeys(request.PostForm, "token", "token_type_hint", "client_id") {
		oauthError(writer, "invalid_request", http.StatusBadRequest)
		return
	}
	token, tokenOK := oneValue(request.PostForm, "token", 128)
	clientID, clientOK := oneValue(request.PostForm, "client_id", 2048)
	hint, hintOK := optionalValue(request.PostForm, "token_type_hint", 32)
	if !tokenOK || !clientOK || !hintOK || hint != "" && hint != "access_token" && hint != "refresh_token" {
		oauthError(writer, "invalid_request", http.StatusBadRequest)
		return
	}
	hash := secretHash(token)
	connections.mu.Lock()
	defer connections.mu.Unlock()
	state := connections.stateLocked()
	for id, grant := range state.Grants {
		if grant.ClientID == clientID && (grant.AccessHash == hash || grant.RefreshHash == hash) {
			delete(state.Grants, id)
		}
	}
	if connections.commitLocked(state) != nil {
		oauthError(writer, "temporarily_unavailable", http.StatusServiceUnavailable)
		return
	}
	writer.WriteHeader(http.StatusOK)
}

func formEncoded(request *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/x-www-form-urlencoded"
}
