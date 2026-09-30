package mcpgateway

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPKCEVerifierAcceptsRFC7636UnreservedCharacters(t *testing.T) {
	for _, verifier := range []string{strings.Repeat("A", 43), strings.Repeat("x", 45), strings.Repeat("a", 124) + "-.~_"} {
		owner := Principal{ID: "owner", Owner: true}
		store := &memoryState{}
		connections := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{owner.ID: owner}}, store)
		digest := sha256.Sum256([]byte(verifier))
		connections.codes[secretHash("code")] = mcpOAuthCode{mcpOAuthRequest: mcpOAuthRequest{
			Client: mcpOAuthClient{ID: "agent"}, RedirectURI: "http://127.0.0.1/callback", Challenge: base64.RawURLEncoding.EncodeToString(digest[:]), ProfileID: owner.ID, Scopes: []string{ReadScope},
		}, Expires: time.Now().Add(time.Minute).Unix()}
		response := httptest.NewRecorder()
		connections.token(response, tokenRequest(t, url.Values{"grant_type": {"authorization_code"}, "client_id": {"agent"}, "resource": {connections.resource}, "code": {"code"}, "redirect_uri": {"http://127.0.0.1/callback"}, "code_verifier": {verifier}}))
		if response.Code != 200 || len(connections.grants) != 1 {
			t.Fatalf("valid verifier rejected: len=%d status=%d", len(verifier), response.Code)
		}
	}
}

func TestS256ChallengeRejectsNoncanonicalAndWrongSizedValues(t *testing.T) {
	owner := Principal{ID: "owner", Owner: true}
	store := &memoryState{}
	connections := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{owner.ID: owner}}, store)
	connections.clients["agent"] = mcpOAuthClient{ID: "agent", Name: "Agent", RedirectURIs: []string{"http://127.0.0.1/callback"}}
	for _, challenge := range []string{"", strings.Repeat("a", 42), strings.Repeat("a", 44), strings.Repeat("a", 128), strings.Repeat("a", 42) + "b", strings.Repeat("a", 42) + "~"} {
		query := url.Values{"response_type": {"code"}, "client_id": {"agent"}, "redirect_uri": {"http://127.0.0.1/callback"}, "resource": {connections.resource}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}}
		response := httptest.NewRecorder()
		connections.authorize(response, principalRequest(t, http.MethodGet, "/oauth/authorize?"+query.Encode(), strings.NewReader(""), owner))
		if response.Code != 400 || len(connections.pending) != 0 || store.saves != 0 {
			t.Fatalf("invalid challenge created authority: length=%d status=%d", len(challenge), response.Code)
		}
	}
}

func refreshAuditToken(t *testing.T, connections *Connections, client, token string) (*httptest.ResponseRecorder, oauthTokens) {
	t.Helper()
	response := httptest.NewRecorder()
	connections.token(response, tokenRequest(t, url.Values{"grant_type": {"refresh_token"}, "client_id": {client}, "resource": {connections.resource}, "refresh_token": {token}}))
	var tokens oauthTokens
	_ = json.Unmarshal(response.Body.Bytes(), &tokens)
	return response, tokens
}

func TestRefreshReplayRevokesEntirePersistedFamily(t *testing.T) {
	owner := Principal{ID: "owner", Owner: true}
	principals := &testPrincipals{values: map[string]Principal{owner.ID: owner}}
	store := &memoryState{}
	connections := testConnections("https://kino.test", principals, store)
	expires := time.Now().Add(time.Hour).Unix()
	connections.grants["grant"] = mcpOAuthGrant{ID: "grant", ClientID: "agent", ProfileID: owner.ID, Scopes: []string{ReadScope}, RefreshHash: secretHash("initial"), RefreshExpires: expires}
	first, rotated := refreshAuditToken(t, connections, "agent", "initial")
	second, current := refreshAuditToken(t, connections, "agent", rotated.RefreshToken)
	if first.Code != 200 || second.Code != 200 {
		t.Fatal("valid refresh failed")
	}
	if connections.grants["grant"].RefreshExpires != expires {
		t.Fatal("rotation extended the grant lifetime")
	}
	before := store.saves
	if response, _ := refreshAuditToken(t, connections, "wrong-client", "initial"); response.Code != 400 || store.saves != before {
		t.Fatal("wrong client changed authority")
	}
	if response, _ := refreshAuditToken(t, connections, "agent", "initial"); response.Code != 400 || len(connections.grants) != 0 {
		t.Fatal("refresh replay retained rotated authority")
	}
	reopened := testConnections("https://kino.test", principals, store)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil)
	if _, err := reopened.VerifyToken(t.Context(), current.AccessToken, request); err == nil {
		t.Fatal("revoked family survived restart")
	}
	if response, _ := refreshAuditToken(t, reopened, "agent", current.RefreshToken); response.Code != 400 {
		t.Fatal("revoked family still refreshes")
	}
}

func TestOAuthRegistrationCapacityRecoversAfterUnusedClientsExpire(t *testing.T) {
	store := &memoryState{}
	connections := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{}}, store)
	now := time.Now()
	connections.now = func() time.Time { return now }
	for index := range 128 {
		id := string(rune(index + 1))
		connections.clients[id] = mcpOAuthClient{ID: id, CreatedAt: now.Add(-25 * time.Hour).Unix()}
	}
	response := httptest.NewRecorder()
	connections.registerClient(response, validRegistrationRequest(t))
	if response.Code != 201 || len(connections.clients) != 1 {
		t.Fatalf("unused registrations permanently block setup: %d clients=%d", response.Code, len(connections.clients))
	}
	connections.grants["expired"] = mcpOAuthGrant{ID: "expired", RefreshExpires: now.Add(-time.Second).Unix(), AccessExpires: now.Add(-time.Second).Unix()}
	connections.clients["active"] = mcpOAuthClient{ID: "active", CreatedAt: now.Add(-25 * time.Hour).Unix()}
	connections.grants["active"] = mcpOAuthGrant{ID: "active", ClientID: "active", RefreshExpires: now.Add(time.Hour).Unix()}
	connections.Views()
	if _, found := connections.grants["expired"]; found {
		t.Fatal("expired grants consume connection slots")
	}
	if _, found := connections.clients["active"]; !found {
		t.Fatal("active connection lost its client registration")
	}
}
