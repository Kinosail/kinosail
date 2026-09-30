package mcpgateway

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func expectEvent(t *testing.T, fixture *eventFixture) []byte {
	t.Helper()
	select {
	case body := <-fixture.received:
		return body
	case <-time.After(3 * time.Second):
		t.Fatal("event not delivered")
		return nil
	}
}

func expectNoEvent(t *testing.T, fixture *eventFixture) {
	t.Helper()
	select {
	case body := <-fixture.received:
		t.Fatalf("unexpected delivery: %s", body)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestEventWorkersRetireAfterQuietExpiration(t *testing.T) {
	fixture := newEventFixture(t)
	fixture.subscribe(t, "library.updated", map[string]any{})
	fixture.connections.mu.Lock()
	for id, subscription := range fixture.connections.events {
		subscription.Expires = time.Now().Add(time.Second).Unix()
		fixture.connections.events[id] = subscription
	}
	fixture.connections.mu.Unlock()
	fixture.gateway.PublishEvent(t.Context(), "", "library.updated", "/api/v1/library")
	expectEvent(t, fixture)
	timer := time.NewTimer(2500 * time.Millisecond)
	defer timer.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		fixture.gateway.eventMu.Lock()
		remaining := len(fixture.gateway.eventRuntime)
		fixture.gateway.eventMu.Unlock()
		if remaining == 0 {
			return
		}
		select {
		case <-ticker.C:
		case <-timer.C:
			t.Fatal("expired quiet subscription retained a worker")
		}
	}
}

func TestEventsUnknownNameReturnsNotFound(t *testing.T) {
	fixture := newEventFixture(t)
	response := mcpRequest(t, fixture.mux, "events/subscribe", map[string]any{"name": "unknown.event", "arguments": map[string]any{}, "delivery": map[string]any{"mode": "webhook", "url": "https://receiver.example.test/hooks", "secret": fixture.secret}})
	if !strings.Contains(response.Body.String(), `"code":-32011`) {
		t.Fatalf("unknown name = %s", response.Body.String())
	}
}

func TestEventsSurviveRestartAndHonorProfileFilters(t *testing.T) {
	fixture := newEventFixture(t)
	first := fixture.subscribe(t, "download.updated", map[string]any{"resource": "/api/v1/downloads/mine"})
	fixture.connections.mu.Lock()
	if len(fixture.connections.events) != 1 {
		t.Fatal("subscription not persisted")
	}
	fixture.connections.mu.Unlock()
	reopened := testConnections("https://kino.test", fixture.principals, fixture.store)
	reopened.client = fixture.connections.client
	if reopened.err != nil {
		t.Fatal(reopened.err)
	}
	mux := http.NewServeMux()
	gateway, err := Register(mux, testGatewayConfig(OAuthConfig{}, fixture.principals, &testAPI{}, testRoutes{}), reopened)
	if err != nil {
		t.Fatal(err)
	}
	fixture.gateway, fixture.connections, fixture.mux = gateway, reopened, mux
	gateway.PublishEvent(t.Context(), "other", "download.updated", "/api/v1/downloads/mine")
	gateway.PublishEvent(t.Context(), "viewer", "download.updated", "/api/v1/downloads/other")
	expectNoEvent(t, fixture)
	gateway.PublishEvent(t.Context(), "viewer", "download.updated", "/api/v1/downloads/mine")
	expectEvent(t, fixture)
	second := fixture.subscribe(t, "download.updated", map[string]any{"resource": "/api/v1/downloads/mine"})
	if first["id"] != second["id"] {
		t.Fatalf("restart changed identity: %v %v", first, second)
	}
}

func TestEventsStopAfterGrantOrProfileRevocationAndExpiry(t *testing.T) {
	for _, cause := range []string{"grant", "profile", "expired"} {
		t.Run(cause, func(t *testing.T) {
			fixture := newEventFixture(t)
			fixture.subscribe(t, "library.updated", map[string]any{})
			switch cause {
			case "grant":
				if err := fixture.connections.Revoke("grant"); err != nil {
					t.Fatal(err)
				}
			case "profile":
				fixture.principals.mu.Lock()
				profile := fixture.principals.values["viewer"]
				profile.Revision++
				fixture.principals.values["viewer"] = profile
				fixture.principals.mu.Unlock()
			case "expired":
				fixture.connections.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
			}
			fixture.gateway.PublishEvent(t.Context(), "", "library.updated", "/api/v1/library")
			expectNoEvent(t, fixture)
		})
	}
}

func TestEventsSecretRotationAndSaveFailure(t *testing.T) {
	fixture := newEventFixture(t)
	first := fixture.subscribe(t, "library.updated", map[string]any{})
	fixture.secret = "whsec_" + base64.StdEncoding.EncodeToString([]byte(strings.Repeat("n", 32)))
	second := fixture.subscribe(t, "library.updated", map[string]any{})
	if first["id"] != second["id"] {
		t.Fatal("rotation changed identity")
	}
	fixture.gateway.PublishEvent(t.Context(), "", "library.updated", "/api/v1/library")
	expectEvent(t, fixture)
	fixture.connections.store = &eventFailStore{memoryState: fixture.store}
	response := mcpRequest(t, fixture.mux, "events/unsubscribe", map[string]any{"name": "library.updated", "arguments": map[string]any{}, "delivery": map[string]any{"mode": "webhook", "url": "https://receiver.example.test/hooks"}})
	if !strings.Contains(response.Body.String(), `"error"`) {
		t.Fatalf("failed save returned success: %s", response.Body.String())
	}
	fixture.connections.store = fixture.store
	fixture.gateway.PublishEvent(t.Context(), "", "library.updated", "/api/v1/library")
	expectEvent(t, fixture)
}

type eventFailStore struct {
	*memoryState
	calls int
}

func (store *eventFailStore) Save(value any) error {
	store.calls++
	if store.calls == 2 {
		return errors.New("storage failed")
	}
	return store.memoryState.Save(value)
}

func TestEventArgumentsRejectDuplicateOrCaseAliasedKeys(t *testing.T) {
	fixture := newEventFixture(t)
	for _, arguments := range []string{`{"resource":"/api/v1/library","resource":"/api/v1/settings"}`, `{"resource":"/api/v1/library","RESOURCE":"/api/v1/library"}`} {
		body := `{"jsonrpc":"2.0","id":1,"method":"events/subscribe","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}},"name":"library.updated","arguments":` + arguments + `,"delivery":{"mode":"webhook","url":"https://receiver.example.test/hooks","secret":` + strconvQuoted(fixture.secret) + `}}}`
		response := rawEventRequest(t, fixture.mux, body)
		if !strings.Contains(response, `"error"`) {
			t.Fatalf("ambiguous arguments accepted: %s", response)
		}
	}
}

func strconvQuoted(value string) string { data, _ := json.Marshal(value); return string(data) }

func rawEventRequest(t *testing.T, handler http.Handler, body string) string {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	request.Header.Set("Mcp-Method", "events/subscribe")
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response.Body.String()
}
