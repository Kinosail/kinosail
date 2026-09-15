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
)

type oauthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func assertOAuthMetadata(t *testing.T, mux *http.ServeMux) {
	t.Helper()
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/.well-known/oauth-authorization-server", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"client_id_metadata_document_supported":true`) || !strings.Contains(response.Body.String(), WriteScope) {
		t.Fatalf("metadata = %d %q", response.Code, response.Body.String())
	}
}

func registerOAuthClient(t *testing.T, mux *http.ServeMux) string {
	t.Helper()
	registration := `{"client_name":"Codex","redirect_uris":["http://127.0.0.1/callback"],"token_endpoint_auth_method":"none","grant_types":["authorization_code","refresh_token"],"response_types":["code"]}`
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/oauth/register", strings.NewReader(registration))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	var registered struct {
		ClientID string `json:"client_id"`
	}
	if response.Code != http.StatusCreated || json.Unmarshal(response.Body.Bytes(), &registered) != nil || registered.ClientID == "" {
		t.Fatalf("registration = %d %q", response.Code, response.Body.String())
	}
	return registered.ClientID
}

func authorizeOAuthClient(t *testing.T, mux *http.ServeMux, connections *Connections, owner Principal, clientID string) (string, string) {
	t.Helper()
	verifier := strings.Repeat("v", 64)
	digest := sha256.Sum256([]byte(verifier))
	query := url.Values{
		"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {"http://127.0.0.1/callback"},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(digest[:])}, "code_challenge_method": {"S256"},
		"resource": {connections.resource}, "scope": {ReadScope + " " + WriteScope}, "state": {"opaque"},
	}
	request := principalRequest(t, http.MethodGet, "/oauth/authorize?"+query.Encode(), strings.NewReader(""), owner)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Allow Codex") {
		t.Fatalf("approval = %d %q", response.Code, response.Body.String())
	}
	pending := htmlFormValue(t, response.Body.String(), "request")
	request = principalRequest(t, http.MethodPost, "/oauth/authorize", strings.NewReader(url.Values{"request": {pending}, "decision": {"allow"}, "scopes": {ReadScope}}.Encode()), owner)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	redirect, err := response.Result().Location()
	if response.Code != http.StatusFound || err != nil || redirect.Query().Get("code") == "" || redirect.Query().Get("state") != "opaque" || redirect.Query().Get("iss") != connections.issuer {
		t.Fatalf("authorization = %d %v %v", response.Code, redirect, err)
	}
	return redirect.Query().Get("code"), verifier
}

func exchangeOAuthCode(t *testing.T, mux *http.ServeMux, connections *Connections, clientID, code, verifier string) oauthTokens {
	t.Helper()
	form := url.Values{"grant_type": {"authorization_code"}, "client_id": {clientID}, "resource": {connections.resource}, "code": {code}, "redirect_uri": {"http://127.0.0.1/callback"}, "code_verifier": {verifier}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	var token oauthTokens
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &token) != nil || token.AccessToken == "" || token.RefreshToken == "" {
		t.Fatalf("token = %d %q", response.Code, response.Body.String())
	}
	return token
}

func assertReopenedGrant(t *testing.T, principals *testPrincipals, store StateStore, owner Principal, accessToken string) { //nolint:cyclop // The score of 12 remains below the repository ceiling of 22 for one reopened-grant assertion.
	t.Helper()
	reopened := testConnections("https://kino.test:38127", principals, store)
	info, err := reopened.VerifyToken(t.Context(), accessToken, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil))
	if err != nil || info.UserID != owner.ID || len(info.Scopes) != 1 || info.Scopes[0] != ReadScope || principals.attributed != owner {
		t.Fatalf("verification = %#v %v attributed=%#v", info, err, principals.attributed)
	}
	views := reopened.Views()
	if len(views) != 1 || views[0].ClientName != "Codex" || views[0].ProfileName != "Owner" || views[0].LastUsed == "" {
		t.Fatalf("views = %#v", views)
	}
	if err := reopened.Revoke(views[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.VerifyToken(t.Context(), accessToken, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil)); err == nil {
		t.Fatal("revoked token remained valid")
	}
}
