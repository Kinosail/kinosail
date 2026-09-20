package mcpgateway

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type memoryState struct {
	data  []byte
	saves int
	err   error
}

func (store *memoryState) Load(target any) (bool, error) {
	if store.err != nil || len(store.data) == 0 {
		return false, store.err
	}
	return true, json.Unmarshal(store.data, target)
}

func (store *memoryState) Save(value any) error {
	if store.err != nil {
		return store.err
	}
	data, err := json.Marshal(value)
	if err == nil {
		store.data, store.saves = data, store.saves+1
	}
	return err
}

func testConnections(issuer string, principals *testPrincipals, store StateStore) *Connections {
	return NewConnections(ConnectionConfig{
		Issuer: issuer, Principals: principals, Store: store,
		SessionKey: func(value string) string { return "key:" + value },
		Error: func(writer http.ResponseWriter, _ *http.Request, err error, status int) {
			writeJSON(writer, map[string]string{"error": err.Error()}, status)
		},
		Approval: func(writer http.ResponseWriter, _ *http.Request, approval Approval) error {
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, err := writer.Write([]byte(`<h1>Allow ` + approval.ClientName + ` to connect?</h1><input name="request" value="` + approval.RequestID + `">`))
			return err
		},
	})
}

func principalRequest(t *testing.T, method, target string, body *strings.Reader, principal Principal) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, target, body)
	return request.WithContext(context.WithValue(request.Context(), testPrincipalKey{}, principal))
}

func TestBuiltInOAuthLifecyclePersistsOnlyHashes(t *testing.T) {
	owner := Principal{ID: "owner", Name: "Owner", Owner: true}
	principals := &testPrincipals{values: map[string]Principal{"owner": owner}}
	store := &memoryState{}
	connections := testConnections("https://kino.test:38127", principals, store)
	mux := http.NewServeMux()
	connections.RegisterOAuth(mux)
	assertOAuthMetadata(t, mux)
	clientID := registerOAuthClient(t, mux)
	code, verifier := authorizeOAuthClient(t, mux, connections, owner, clientID)
	token := exchangeOAuthCode(t, mux, connections, clientID, code, verifier)
	if bytes.Contains(store.data, []byte(token.AccessToken)) || bytes.Contains(store.data, []byte(token.RefreshToken)) {
		t.Fatalf("persisted bearer credentials: %s", store.data)
	}
	assertReopenedGrant(t, principals, store, owner, token.AccessToken)
}

func TestBuiltInOAuthRejectsMalformedUnknownAndOversizedInputWithoutAuthority(t *testing.T) { //nolint:cyclop,funlen // Rejected inputs must not persist authority.
	viewer := Principal{ID: "viewer", Name: "Viewer"}
	principals := &testPrincipals{values: map[string]Principal{"viewer": viewer}}
	store := &memoryState{}
	connections := testConnections("https://kino.test:38127", principals, store)
	mux := http.NewServeMux()
	connections.RegisterOAuth(mux)
	for name, body := range map[string]string{
		"duplicate":       `{"client_name":"Old","client_name":"Agent","redirect_uris":["http://127.0.0.1/callback"]}`,
		"duplicate alias": `{"CLIENT_NAME":"Old","client_name":"Agent","redirect_uris":["http://127.0.0.1/callback"]}`,
		"unknown":         `{"client_name":"Agent","redirect_uris":["http://127.0.0.1/callback"],"unknown":true}`,
		"remote redirect": `{"client_name":"Agent","redirect_uris":["http://example.com/callback"]}`,
		"oversized":       `{"client_name":"` + strings.Repeat("x", 65<<10) + `","redirect_uris":["http://127.0.0.1/callback"]}`,
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/oauth/register", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest || len(connections.clients) != 0 || len(connections.grants) != 0 || store.saves != 0 {
				t.Fatalf("registration = %d clients=%d grants=%d saves=%d", response.Code, len(connections.clients), len(connections.grants), store.saves)
			}
		})
	}
	connections.clients["agent"] = mcpOAuthClient{ID: "agent", Name: "Agent", RedirectURIs: []string{"http://127.0.0.1/callback"}}
	query := url.Values{"response_type": {"code"}, "client_id": {"agent", "duplicate"}, "redirect_uri": {"http://127.0.0.1/callback"}, "code_challenge": {strings.Repeat("a", 43)}, "code_challenge_method": {"S256"}, "resource": {connections.resource}}
	request := principalRequest(t, http.MethodGet, "/oauth/authorize?"+query.Encode(), strings.NewReader(""), viewer)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || len(connections.pending) != 0 || store.saves != 0 {
		t.Fatalf("ambiguous authorization = %d pending=%d saves=%d", response.Code, len(connections.pending), store.saves)
	}
	query.Set("client_id", "agent")
	query.Set("unknown", "true")
	request = principalRequest(t, http.MethodGet, "/oauth/authorize?"+query.Encode(), strings.NewReader(""), viewer)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || len(connections.pending) != 0 || store.saves != 0 {
		t.Fatalf("unknown authorization = %d pending=%d", response.Code, len(connections.pending))
	}
	pending := rand.Text()
	connections.pending["key:"+pending] = mcpOAuthRequest{Client: connections.clients["agent"], RedirectURI: "http://127.0.0.1/callback", Scopes: []string{ReadScope}, ProfileID: viewer.ID, Expires: time.Now().Add(time.Minute).Unix()}
	request = principalRequest(t, http.MethodPost, "/oauth/authorize", strings.NewReader(url.Values{"request": {pending}, "decision": {"allow"}, "scopes": {ReadScope, WriteScope}}.Encode()), viewer)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || len(connections.pending) != 0 || len(connections.codes) != 0 || len(connections.grants) != 0 || store.saves != 0 {
		t.Fatalf("scope escalation = %d pending=%d codes=%d grants=%d saves=%d", response.Code, len(connections.pending), len(connections.codes), len(connections.grants), store.saves)
	}
	code := rand.Text()
	connections.codes[secretHash(code)] = mcpOAuthCode{mcpOAuthRequest: mcpOAuthRequest{Client: connections.clients["agent"], RedirectURI: "http://127.0.0.1/callback", Challenge: strings.Repeat("a", 43), Scopes: []string{ReadScope}, ProfileID: viewer.ID}, Expires: time.Now().Add(time.Minute).Unix()}
	form := url.Values{"grant_type": {"authorization_code"}, "client_id": {"agent"}, "resource": {connections.resource}, "code": {code}, "redirect_uri": {"http://127.0.0.1/callback"}, "code_verifier": {strings.Repeat("v", 64)}}
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || len(connections.codes) != 0 || len(connections.grants) != 0 || store.saves != 0 {
		t.Fatalf("invalid verifier = %d codes=%d grants=%d saves=%d", response.Code, len(connections.codes), len(connections.grants), store.saves)
	}
}

func TestOAuthRefreshRevocationAndManagementApproval(t *testing.T) { //nolint:cyclop,funlen,gocognit // Rotation, revocation, and recent-auth checks remain atomic.
	owner := Principal{ID: "owner", Name: "Owner", Owner: true}
	principals := &testPrincipals{values: map[string]Principal{"owner": owner}}
	store := &memoryState{}
	connections := testConnections("https://kino.test:38127", principals, store)
	oldAccess, oldRefresh := "old-access", "old-refresh"
	connections.grants["grant"] = mcpOAuthGrant{ID: "grant", ClientID: "agent", ProfileID: owner.ID, Scopes: []string{ReadScope}, AccessHash: secretHash(oldAccess), AccessExpires: time.Now().Add(time.Hour).Unix(), RefreshHash: secretHash(oldRefresh), RefreshExpires: time.Now().Add(time.Hour).Unix()}
	mux := http.NewServeMux()
	connections.RegisterOAuth(mux)
	refresh := func(value string) *httptest.ResponseRecorder {
		form := url.Values{"grant_type": {"refresh_token"}, "client_id": {"agent"}, "resource": {connections.resource}, "refresh_token": {value}}
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		return response
	}
	response := refresh(oldRefresh)
	var rotated struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &rotated) != nil || rotated.AccessToken == "" {
		t.Fatalf("refresh = %d %q", response.Code, response.Body.String())
	}
	if replay := refresh(oldRefresh); replay.Code != http.StatusBadRequest {
		t.Fatalf("refresh replay = %d", replay.Code)
	}
	if _, err := connections.VerifyToken(t.Context(), oldAccess, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil)); err == nil {
		t.Fatal("old access remained valid")
	}
	if _, err := connections.VerifyToken(t.Context(), rotated.AccessToken, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil)); err != nil {
		t.Fatalf("rotated access = %v", err)
	}
	for name, form := range map[string]url.Values{
		"unknown field": {"token": {rotated.AccessToken}, "client_id": {"agent"}, "extra": {"true"}},
		"duplicate":     {"token": {rotated.AccessToken, rotated.AccessToken}, "client_id": {"agent"}},
		"oversized":     {"token": {strings.Repeat("x", 129)}, "client_id": {"agent"}},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/oauth/revoke", strings.NewReader(form.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			result := httptest.NewRecorder()
			mux.ServeHTTP(result, request)
			if result.Code != http.StatusBadRequest || len(connections.grants) != 1 {
				t.Fatalf("revocation = %d grants=%d", result.Code, len(connections.grants))
			}
		})
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/oauth/revoke", strings.NewReader(url.Values{"token": {rotated.AccessToken}, "client_id": {"agent"}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || len(connections.grants) != 0 {
		t.Fatalf("valid revocation = %d grants=%d", response.Code, len(connections.grants))
	}
	connections.clients["agent"] = mcpOAuthClient{ID: "agent", Name: "Agent", RedirectURIs: []string{"http://127.0.0.1/callback"}}
	requestID := "request"
	connections.pending["key:"+requestID] = mcpOAuthRequest{Client: connections.clients["agent"], RedirectURI: "http://127.0.0.1/callback", Scopes: []string{ReadScope, ManageScope}, ProfileID: owner.ID, Expires: time.Now().Add(time.Minute).Unix()}
	approve := func() *httptest.ResponseRecorder {
		request := principalRequest(t, http.MethodPost, "/oauth/authorize", strings.NewReader(url.Values{"request": {requestID}, "decision": {"allow"}, "scopes": {ReadScope, ManageScope}}.Encode()), owner)
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		result := httptest.NewRecorder()
		mux.ServeHTTP(result, request)
		return result
	}
	if result := approve(); result.Code != http.StatusForbidden || len(connections.codes) != 0 {
		t.Fatalf("stale management approval = %d codes=%d", result.Code, len(connections.codes))
	}
	connections.pending["key:"+requestID] = mcpOAuthRequest{Client: connections.clients["agent"], RedirectURI: "http://127.0.0.1/callback", Scopes: []string{ReadScope, ManageScope}, ProfileID: owner.ID, Expires: time.Now().Add(time.Minute).Unix()}
	principals.recent = true
	if result := approve(); result.Code != http.StatusFound || len(connections.codes) != 1 {
		t.Fatalf("recent management approval = %d codes=%d", result.Code, len(connections.codes))
	}
}

func TestConnectionsFailClosedOnStateAndMetadataErrors(t *testing.T) {
	principals := &testPrincipals{values: map[string]Principal{}}
	connections := testConnections("http://public.example", principals, &memoryState{})
	mux := http.NewServeMux()
	connections.RegisterOAuth(mux)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/.well-known/oauth-authorization-server", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("invalid issuer metadata = %d", response.Code)
	}
	failed := testConnections("https://kino.test", principals, &memoryState{err: errors.New("disk failed")})
	if failed.err == nil {
		t.Fatal("state load error was ignored")
	}
	if err := failed.Revoke("missing"); err == nil {
		t.Fatal("unknown grant was revoked")
	}
}
