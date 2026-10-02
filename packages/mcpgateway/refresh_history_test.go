package mcpgateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPersistedRefreshHistoryIsBoundedAndUnambiguous(t *testing.T) {
	owner := Principal{ID: "owner", Owner: true}
	principals := &testPrincipals{values: map[string]Principal{owner.ID: owner}}
	for _, history := range [][]string{make([]string, 1025), {"not-a-hash"}, {secretHash("old"), secretHash("old")}, {strings.ToUpper(secretHash("old"))}} {
		grant := mcpOAuthGrant{ID: "grant", ProfileID: owner.ID, Scopes: []string{ReadScope}, AccessHash: secretHash("access"), AccessExpires: time.Now().Add(time.Hour).Unix(), RefreshHistory: history}
		data, _ := json.Marshal(mcpConnectionState{Grants: map[string]mcpOAuthGrant{grant.ID: grant}})
		store := &memoryState{data: data}
		connections := testConnections("https://kino.test", principals, store)
		if info, err := connections.VerifyToken(t.Context(), "access", httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil)); err == nil || info != nil || store.saves != 0 {
			t.Fatal("invalid persisted history created authority")
		}
	}
}

func TestRefreshReplayFailsClosedWhenRevocationCannotPersist(t *testing.T) {
	owner := Principal{ID: "owner", Owner: true}
	principals := &testPrincipals{values: map[string]Principal{owner.ID: owner}}
	store := &memoryState{}
	connections := testConnections("https://kino.test", principals, store)
	connections.grants["grant"] = mcpOAuthGrant{ID: "grant", ClientID: "agent", ProfileID: owner.ID, Scopes: []string{ReadScope}, RefreshHash: secretHash("initial"), RefreshExpires: time.Now().Add(time.Hour).Unix()}
	response, tokens := refreshAuditToken(t, connections, "agent", "initial")
	if response.Code != 200 {
		t.Fatal("valid refresh rejected")
	}
	store.err = http.ErrServerClosed
	response, _ = refreshAuditToken(t, connections, "agent", "initial")
	if response.Code != 503 {
		t.Fatalf("failed persistence reported success: %d", response.Code)
	}
	if info, err := connections.VerifyToken(t.Context(), tokens.AccessToken, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil)); err == nil || info != nil {
		t.Fatal("replayed family remains valid in memory")
	}
}

func TestRefreshReportsActualRemainingAccessLifetime(t *testing.T) {
	owner := Principal{ID: "owner", Owner: true}
	connections := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{owner.ID: owner}}, &memoryState{})
	now := time.Now().Truncate(time.Second)
	connections.now = func() time.Time { return now }
	connections.grants["grant"] = mcpOAuthGrant{ID: "grant", ClientID: "agent", ProfileID: owner.ID, Scopes: []string{ReadScope}, RefreshHash: secretHash("initial"), RefreshExpires: now.Add(time.Minute).Unix()}
	response, tokens := refreshAuditToken(t, connections, "agent", "initial")
	var result struct {
		ExpiresIn int `json:"expires_in"`
	}
	if json.Unmarshal(response.Body.Bytes(), &result) != nil || response.Code != 200 || result.ExpiresIn != 60 {
		t.Fatalf("incorrect lifetime: %d %s", response.Code, response.Body.String())
	}
	now = now.Add(time.Minute)
	if response, _ := refreshAuditToken(t, connections, "agent", tokens.RefreshToken); response.Code != 400 {
		t.Fatal("expired family created another access token")
	}
}

func TestUnknownTokenRevocationCreatesNoPersistenceEffects(t *testing.T) {
	store := &memoryState{}
	connections := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{}}, store)
	request := tokenRequest(t, url.Values{"token": {"unknown"}, "client_id": {"unknown"}})
	request.URL.Path = "/oauth/revoke"
	response := httptest.NewRecorder()
	connections.revokeToken(response, request)
	if response.Code != 200 || store.saves != 0 {
		t.Fatalf("unknown revoke rewrote state: status=%d saves=%d", response.Code, store.saves)
	}
}
