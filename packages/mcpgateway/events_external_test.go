package mcpgateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExternalEventsRequireBoundedClientIdentityAndExpireWithToken(t *testing.T) { //nolint:cyclop,gocognit // The introspected-token journey owns rejection and expiry behavior for every client case.
	for _, clientID := range []string{"", strings.Repeat("x", 2049), "external-client"} {
		t.Run(clientID[:min(len(clientID), 16)], func(t *testing.T) {
			fixture := newEventFixture(t)
			expiration := time.Now().Add(2 * time.Minute).Unix()
			issuer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(writer).Encode(map[string]any{"active": true, "scope": ReadScope, "exp": expiration, "sub": "subject", "aud": "http://localhost/mcp", "client_id": clientID})
			}))
			defer issuer.Close()
			fixture.principals.oidc = map[string]Principal{issuer.URL + "\x00subject": fixture.principals.values["viewer"]}
			config := testGatewayConfig(OAuthConfig{ResourceURL: "http://localhost/mcp", AuthorizationServer: issuer.URL, IntrospectionURL: issuer.URL, ClientID: "server", ClientSecret: "test-only"}, fixture.principals, &testAPI{}, testRoutes{})
			fixture.mux = http.NewServeMux()
			var err error
			fixture.gateway, err = Register(fixture.mux, config, fixture.connections)
			if err != nil {
				t.Fatal(err)
			}
			fields := map[string]any{"name": "library.updated", "arguments": map[string]any{}, "delivery": map[string]any{"mode": "webhook", "url": "https://receiver.example.test/hooks", "secret": fixture.secret}}
			response := mcpRequest(t, fixture.mux, "events/subscribe", fields)
			if clientID == "" {
				if !strings.Contains(response.Body.String(), `"code":-32014`) {
					t.Fatalf("missing client identity = %s", response.Body.String())
				}
				return
			}
			if len(clientID) > 2048 {
				if response.Code != http.StatusUnauthorized || fixture.verifications.Load() != 0 {
					t.Fatalf("oversized identity = %d %s", response.Code, response.Body.String())
				}
				return
			}
			if strings.Contains(response.Body.String(), `"error"`) {
				t.Fatalf("external subscribe = %s", response.Body.String())
			}
			fixture.connections.mu.Lock()
			for _, subscription := range fixture.connections.events {
				if subscription.Expires != expiration {
					t.Errorf("external subscription outlived access token: %d %d", subscription.Expires, expiration)
				}
			}
			fixture.connections.mu.Unlock()
			fixture.gateway.PublishEvent(t.Context(), "", "library.updated", "/api/v1/library")
			expectEvent(t, fixture)
			fixture.connections.mu.Lock()
			for id, subscription := range fixture.connections.events {
				subscription.AuthorityExpires = time.Now().Add(-time.Second).Unix()
				fixture.connections.events[id] = subscription
			}
			fixture.connections.mu.Unlock()
			fixture.gateway.PublishEvent(t.Context(), "", "library.updated", "/api/v1/library")
			expectNoEvent(t, fixture)
		})
	}
}
