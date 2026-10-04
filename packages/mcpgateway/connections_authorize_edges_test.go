package mcpgateway

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func validAuthorizationQuery(connections *Connections, clientID, scope string) string {
	return url.Values{
		"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {"http://127.0.0.1/callback"},
		"code_challenge": {strings.Repeat("A", 43)}, "code_challenge_method": {"S256"},
		"resource": {connections.resource}, "scope": {scope},
	}.Encode()
}

func TestConnectionsDefaultIssuerAndPendingLimit(t *testing.T) {
	viewer := Principal{ID: "viewer", Name: "Viewer"}
	principals := &testPrincipals{values: map[string]Principal{"viewer": viewer}}
	connections := testConnections("", principals, &memoryState{})
	if connections.issuer != "http://localhost" || connections.resource != "http://localhost/mcp" || connections.err != nil {
		t.Fatalf("default connections = %#v", connections)
	}
	connections.clients["agent"] = mcpOAuthClient{ID: "agent", Name: "Agent", RedirectURIs: []string{"http://127.0.0.1/callback"}}
	now := time.Unix(1_800_000_000, 0)
	connections.now = func() time.Time { return now }
	for index := range mcpConnectionLimit {
		connections.pending[string(rune(index))] = mcpOAuthRequest{Expires: now.Add(time.Minute).Unix()}
	}
	request := principalRequest(t, http.MethodGet, "/oauth/authorize?"+validAuthorizationQuery(connections, "agent", ReadScope), strings.NewReader(""), viewer)
	response := httptest.NewRecorder()
	connections.authorize(response, request)
	if response.Code != http.StatusTooManyRequests || len(connections.pending) != mcpConnectionLimit {
		t.Fatalf("pending limit = %d, pending=%d", response.Code, len(connections.pending))
	}
}

func TestAuthorizationRequestRejectsClientAndScopeBeforePendingState(t *testing.T) {
	viewer := Principal{ID: "viewer", Name: "Viewer"}
	connections := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{"viewer": viewer}}, &memoryState{})
	connections.clients["agent"] = mcpOAuthClient{ID: "agent", Name: "Agent", RedirectURIs: []string{"http://127.0.0.1/callback"}}
	for name, query := range map[string]string{
		"unknown client": validAuthorizationQuery(connections, "missing", ReadScope),
		"invalid scope":  validAuthorizationQuery(connections, "agent", "unknown"),
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/oauth/authorize?"+query, nil)
			if _, err := connections.authorizationRequest(request, viewer); err == nil {
				t.Fatal("invalid authorization request succeeded")
			}
			if len(connections.pending) != 0 {
				t.Fatal("invalid authorization created pending authority")
			}
		})
	}
	if scopes, err := normalizeMCPScopes("", true); err != nil || len(scopes) != 1 || scopes[0] != ReadScope {
		t.Fatalf("default scopes = %#v, %v", scopes, err)
	}
	if validPKCEValue("short") {
		t.Fatal("short PKCE value was accepted")
	}
}

func TestMetadataURLAndRedirectBounds(t *testing.T) {
	for _, id := range []string{strings.Repeat("x", 2049), ":", "http://example.test/client", "https://example.test/"} {
		if validClientMetadataURL(id) {
			t.Fatalf("metadata URL %q was accepted", id)
		}
	}
	if redirectAllowed([]string{"http://127.0.0.1/callback"}, "not-a-redirect") {
		t.Fatal("invalid redirect was accepted")
	}
}

func TestApprovalRejectsMalformedOrMissingPendingRequestWithoutCode(t *testing.T) {
	viewer := Principal{ID: "viewer", Name: "Viewer"}
	connections := testConnections("https://kino.test", &testPrincipals{values: map[string]Principal{"viewer": viewer}}, &memoryState{})
	malformed := principalRequest(t, http.MethodPost, "/oauth/authorize", strings.NewReader("request=x"), viewer)
	response := httptest.NewRecorder()
	connections.approve(response, malformed)
	if response.Code != http.StatusBadRequest || len(connections.codes) != 0 {
		t.Fatalf("malformed approval = %d, codes=%d", response.Code, len(connections.codes))
	}
	missing := principalRequest(t, http.MethodPost, "/oauth/authorize", strings.NewReader(url.Values{"request": {"missing"}, "decision": {"allow"}, "scopes": {ReadScope}}.Encode()), viewer)
	missing.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response = httptest.NewRecorder()
	connections.approve(response, missing)
	if response.Code != http.StatusBadRequest || len(connections.codes) != 0 {
		t.Fatalf("missing approval = %d, codes=%d", response.Code, len(connections.codes))
	}
	if scopes, ok := connections.approvedScopes(missing, mcpOAuthRequest{}, viewer, nil); ok || scopes != nil {
		t.Fatalf("empty approved scopes = %#v, %t", scopes, ok)
	}
}
