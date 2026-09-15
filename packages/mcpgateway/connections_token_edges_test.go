package mcpgateway

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func tokenRequest(t *testing.T, values url.Values) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/oauth/token", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func TestTokenBoundaryFailuresCreateNoGrant(t *testing.T) { //nolint:cyclop // The score of 13 remains below the repository ceiling of 22 for the no-authority boundary matrix.
	store := &memoryState{}
	connections := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{}}, store)
	wrongType := tokenRequest(t, url.Values{})
	wrongType.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	connections.token(response, wrongType)
	if response.Code != http.StatusBadRequest || len(connections.grants) != 0 || store.saves != 0 {
		t.Fatalf("wrong type token = %d grants=%d saves=%d", response.Code, len(connections.grants), store.saves)
	}
	oversized := tokenRequest(t, url.Values{"grant_type": {strings.Repeat("x", (64<<10)+1)}})
	oversized.ContentLength = -1
	response = httptest.NewRecorder()
	connections.token(response, oversized)
	if response.Code != http.StatusBadRequest || len(connections.grants) != 0 || store.saves != 0 {
		t.Fatalf("oversized token = %d grants=%d saves=%d", response.Code, len(connections.grants), store.saves)
	}
	missing := tokenRequest(t, url.Values{"grant_type": {"authorization_code"}})
	response = httptest.NewRecorder()
	connections.token(response, missing)
	if response.Code != http.StatusBadRequest || len(connections.grants) != 0 || store.saves != 0 {
		t.Fatalf("missing token fields = %d grants=%d saves=%d", response.Code, len(connections.grants), store.saves)
	}
	unsupported := tokenRequest(t, url.Values{"grant_type": {"password"}, "client_id": {"agent"}, "resource": {connections.resource}})
	response = httptest.NewRecorder()
	connections.token(response, unsupported)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "unsupported_grant_type") || len(connections.grants) != 0 {
		t.Fatalf("unsupported token = %d %q", response.Code, response.Body.String())
	}
}

func TestCodeExchangeRejectsMalformedAndUnauthorizedTransactions(t *testing.T) {
	connections := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{}}, &memoryState{})
	malformed := tokenRequest(t, nil)
	malformed.PostForm = url.Values{"code": {"short"}}
	response := httptest.NewRecorder()
	connections.exchangeCode(response, malformed, "agent")
	if response.Code != http.StatusBadRequest || len(connections.grants) != 0 {
		t.Fatalf("malformed exchange = %d grants=%d", response.Code, len(connections.grants))
	}
	verifier := strings.Repeat("v", 64)
	digest := sha256.Sum256([]byte(verifier))
	code := "one-use-code"
	connections.codes[secretHash(code)] = mcpOAuthCode{mcpOAuthRequest: mcpOAuthRequest{
		Client: mcpOAuthClient{ID: "agent"}, RedirectURI: "http://127.0.0.1/callback", Challenge: base64.RawURLEncoding.EncodeToString(digest[:]),
		Scopes: []string{ReadScope}, ProfileID: "missing",
	}, Expires: time.Now().Add(time.Minute).Unix()}
	request := tokenRequest(t, nil)
	request.PostForm = url.Values{
		"grant_type": {"authorization_code"}, "client_id": {"agent"}, "resource": {connections.resource}, "code": {code},
		"redirect_uri": {"http://127.0.0.1/callback"}, "code_verifier": {verifier},
	}
	response = httptest.NewRecorder()
	connections.exchangeCode(response, request, "agent")
	if response.Code != http.StatusBadRequest || len(connections.codes) != 0 || len(connections.grants) != 0 {
		t.Fatalf("unauthorized exchange = %d codes=%d grants=%d", response.Code, len(connections.codes), len(connections.grants))
	}
}

func TestRefreshRejectsMalformedUnauthorizedAndUnpersistedRotations(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	connections := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{}}, &memoryState{})
	connections.now = func() time.Time { return now }
	malformed := tokenRequest(t, nil)
	malformed.PostForm = url.Values{"refresh_token": {"token"}, "unknown": {"true"}}
	response := httptest.NewRecorder()
	connections.refresh(response, malformed, "agent")
	if response.Code != http.StatusBadRequest || len(connections.grants) != 0 {
		t.Fatalf("malformed refresh = %d grants=%d", response.Code, len(connections.grants))
	}
	refresh := "refresh-token"
	grant := mcpOAuthGrant{ID: "grant", ClientID: "agent", ProfileID: "missing", RefreshHash: secretHash(refresh), RefreshExpires: now.Add(time.Hour).Unix()}
	connections.grants[grant.ID] = grant
	request := tokenRequest(t, nil)
	request.PostForm = url.Values{"grant_type": {"refresh_token"}, "client_id": {"agent"}, "resource": {connections.resource}, "refresh_token": {refresh}}
	response = httptest.NewRecorder()
	connections.refresh(response, request, "agent")
	if response.Code != http.StatusBadRequest || !reflect.DeepEqual(connections.grants[grant.ID], grant) {
		t.Fatalf("unauthorized refresh = %d grant=%#v", response.Code, connections.grants[grant.ID])
	}
	profile := Principal{ID: "viewer", Name: "Viewer"}
	connections.principals = &testPrincipals{values: map[string]Principal{profile.ID: profile}}
	grant.ProfileID = profile.ID
	connections.grants[grant.ID] = grant
	connections.store = &saveErrorStore{err: errTestSave}
	response = httptest.NewRecorder()
	connections.refresh(response, request, "agent")
	if response.Code != http.StatusServiceUnavailable || !reflect.DeepEqual(connections.grants[grant.ID], grant) {
		t.Fatalf("unpersisted refresh = %d grant=%#v", response.Code, connections.grants[grant.ID])
	}
}

var errTestSave = errors.New("save failed")

func TestTokenIssueLimitsAndPersistenceFailureCreateNoGrant(t *testing.T) {
	connections := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{}}, &memoryState{})
	for index := range mcpConnectionLimit {
		connections.grants[string(rune(index))] = mcpOAuthGrant{}
	}
	response := httptest.NewRecorder()
	connections.issue(response, mcpOAuthClient{ID: "agent"}, "viewer", []string{ReadScope})
	if response.Code != http.StatusTooManyRequests || len(connections.grants) != mcpConnectionLimit {
		t.Fatalf("grant limit = %d grants=%d", response.Code, len(connections.grants))
	}
	failed := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{}}, &memoryState{})
	failed.store = &saveErrorStore{err: errTestSave}
	response = httptest.NewRecorder()
	failed.issue(response, mcpOAuthClient{ID: "agent"}, "viewer", []string{ReadScope})
	if response.Code != http.StatusServiceUnavailable || len(failed.grants) != 0 {
		t.Fatalf("failed grant issue = %d grants=%d", response.Code, len(failed.grants))
	}
}

func TestVerifyAndRevokePersistenceFailuresPreserveGrant(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	profile := Principal{ID: "viewer", Name: "Viewer"}
	principals := &testPrincipals{values: map[string]Principal{profile.ID: profile}}
	connections := testConnections("https://kino.test", principals, &memoryState{})
	connections.now = func() time.Time { return now }
	access := "access-token"
	grant := mcpOAuthGrant{ID: "grant", ClientID: "agent", ProfileID: profile.ID, AccessHash: secretHash(access), AccessExpires: now.Add(time.Hour).Unix(), RefreshHash: secretHash("refresh"), RefreshExpires: now.Add(time.Hour).Unix()}
	unauthorized := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{}}, &memoryState{})
	unauthorized.now = func() time.Time { return now }
	unauthorized.grants[grant.ID] = grant
	if _, err := unauthorized.VerifyToken(t.Context(), access, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil)); err == nil || !reflect.DeepEqual(unauthorized.grants[grant.ID], grant) {
		t.Fatalf("unauthorized token verification = %v grant=%#v", err, unauthorized.grants[grant.ID])
	}
	connections.grants[grant.ID] = grant
	connections.store = &saveErrorStore{err: errTestSave}
	if _, err := connections.VerifyToken(t.Context(), access, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil)); err == nil || !reflect.DeepEqual(connections.grants[grant.ID], grant) {
		t.Fatalf("failed token verification = %v grant=%#v", err, connections.grants[grant.ID])
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/oauth/revoke", strings.NewReader(url.Values{"token": {access}, "client_id": {"agent"}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	connections.revokeToken(response, request)
	if response.Code != http.StatusServiceUnavailable || !reflect.DeepEqual(connections.grants[grant.ID], grant) {
		t.Fatalf("failed token revoke = %d grant=%#v", response.Code, connections.grants[grant.ID])
	}
}
