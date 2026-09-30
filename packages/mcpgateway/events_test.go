package mcpgateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/liveevents"
)

// This HTTP contract owns event discovery and lifecycle coverage. Existing tool
// tests cannot detect absent event methods, forged callbacks, leaked profile
// events, forgotten subscriptions after restart, or delivery after revocation.
func TestEventsRPCDiscovery(t *testing.T) {
	principal := Principal{ID: "viewer", Name: "Viewer", Revision: 1}
	principals := &testPrincipals{values: map[string]Principal{principal.ID: principal}}
	connections := testConnections("https://kino.test", principals, &memoryState{})
	connections.clients["client"] = mcpOAuthClient{ID: "client", Name: "Agent", RedirectURIs: []string{"http://127.0.0.1/callback"}, CreatedAt: time.Now().Unix()}
	connections.grants["grant"] = mcpOAuthGrant{ID: "grant", ClientID: "client", ClientName: "Agent", ProfileID: principal.ID, ProfileRevision: principal.Revision, Scopes: []string{ReadScope}, CreatedAt: time.Now().Unix(), AccessHash: secretHash("token"), AccessExpires: time.Now().Add(time.Hour).Unix(), RefreshHash: secretHash("refresh"), RefreshExpires: time.Now().Add(time.Hour).Unix()}
	mux := http.NewServeMux()
	_, err := Register(mux, testGatewayConfig(OAuthConfig{}, principals, &testAPI{}, testRoutes{}), connections)
	if err != nil {
		t.Fatal(err)
	}
	response := mcpRequest(t, mux, "server/discover", nil)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"events":{}`) {
		t.Fatalf("event discovery = %d %s", response.Code, response.Body.String())
	}
	response = mcpRequest(t, mux, "events/list", nil)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"name":"library.updated"`) || !strings.Contains(response.Body.String(), `"webhook"`) {
		t.Fatalf("event list = %d %s", response.Code, response.Body.String())
	}
}

type eventTestTransport struct {
	destination *url.URL
	transport   http.RoundTripper
}

func (transport eventTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	cloned := request.Clone(request.Context())
	destination := *request.URL
	destination.Host = transport.destination.Host
	cloned.URL = &destination
	return transport.transport.RoundTrip(cloned)
}

type eventFixture struct {
	mux           *http.ServeMux
	gateway       *Gateway
	connections   *Connections
	store         *memoryState
	principals    *testPrincipals
	received      chan []byte
	secret        string
	verifications atomic.Int64
	status        atomic.Int64
	replyGate     atomic.Pointer[eventReplyGate]
}

type eventReplyGate struct {
	arrived chan struct{}
	resume  chan struct{}
}

func newEventFixture(t *testing.T) *eventFixture { //nolint:cyclop,gocognit // The real TLS callback validates signing, challenges, redirects, and delivery in one fixture.
	t.Helper()
	principal := Principal{ID: "viewer", Name: "Viewer", Revision: 1}
	fixture := &eventFixture{store: &memoryState{}, principals: &testPrincipals{values: map[string]Principal{"viewer": principal}}, received: make(chan []byte, 32), secret: "whsec_" + base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))}
	fixture.connections = testConnections("https://kino.test", fixture.principals, fixture.store)
	fixture.connections.clients["client"] = mcpOAuthClient{ID: "client", Name: "Agent", RedirectURIs: []string{"http://127.0.0.1/callback"}, CreatedAt: time.Now().Unix()}
	fixture.connections.grants["grant"] = mcpOAuthGrant{ID: "grant", ClientID: "client", ClientName: "Agent", ProfileID: "viewer", ProfileRevision: 1, Scopes: []string{ReadScope}, CreatedAt: time.Now().Unix(), AccessHash: secretHash("token"), AccessExpires: time.Now().Add(time.Hour).Unix(), RefreshHash: secretHash("refresh"), RefreshExpires: time.Now().Add(time.Hour).Unix()}
	receiver := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		key, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(fixture.secret, "whsec_"))
		signer := hmac.New(sha256.New, key)
		_, _ = signer.Write([]byte(request.Header.Get("webhook-id") + "." + request.Header.Get("webhook-timestamp") + "."))
		_, _ = signer.Write(body)
		expected := "v1," + base64.StdEncoding.EncodeToString(signer.Sum(nil))
		if !strings.Contains(request.Header.Get("webhook-signature"), expected) || request.Header.Get("X-MCP-Subscription-Id") == "" || request.Header.Get("Authorization") != "" {
			t.Errorf("invalid webhook headers: %v", request.Header)
		}
		var control struct {
			Type      string `json:"type"`
			Challenge string `json:"challenge"`
			EventID   string `json:"eventId"`
		}
		_ = json.Unmarshal(body, &control)
		if control.Type == "verification" {
			fixture.verifications.Add(1)
			if request.URL.Path == "/bad-challenge" {
				_ = json.NewEncoder(writer).Encode(map[string]string{"challenge": "wrong"})
				return
			}
			if request.URL.Path == "/redirect" {
				writer.Header().Set("Location", "https://127.0.0.1/internal")
				writer.WriteHeader(http.StatusTemporaryRedirect)
				return
			}
			_ = json.NewEncoder(writer).Encode(map[string]string{"challenge": control.Challenge})
			return
		}
		if control.EventID != request.Header.Get("webhook-id") {
			t.Errorf("event ID/header mismatch: %s", body)
		}
		status := int(fixture.status.Load())
		if status == 0 {
			status = http.StatusNoContent
		}
		fixture.received <- body
		if gate := fixture.replyGate.Swap(nil); gate != nil {
			close(gate.arrived)
			<-gate.resume
		}
		writer.WriteHeader(status)
	}))
	t.Cleanup(receiver.Close)
	destination, _ := url.Parse(receiver.URL)
	fixture.connections.client = &http.Client{Transport: eventTestTransport{destination, receiver.Client().Transport}, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	fixture.mux = http.NewServeMux()
	var err error
	fixture.gateway, err = Register(fixture.mux, testGatewayConfig(OAuthConfig{}, fixture.principals, &testAPI{}, testRoutes{}), fixture.connections)
	if err != nil {
		t.Fatal(err)
	}
	return fixture
}

func (fixture *eventFixture) subscribe(t *testing.T, name string, arguments map[string]any) map[string]any {
	t.Helper()
	response := mcpRequest(t, fixture.mux, "events/subscribe", map[string]any{"name": name, "arguments": arguments, "delivery": map[string]any{"mode": "webhook", "url": "https://receiver.example.test/hooks", "secret": fixture.secret}, "cursor": nil, "ttlMs": 60000})
	var envelope struct {
		Result map[string]any `json:"result"`
		Error  any            `json:"error"`
	}
	if json.Unmarshal(response.Body.Bytes(), &envelope) != nil || envelope.Error != nil || envelope.Result["id"] == nil {
		t.Fatalf("subscribe = %d %s", response.Code, response.Body.String())
	}
	return envelope.Result
}

func TestEventsWebhookLifecycle(t *testing.T) { //nolint:cyclop // One public HTTP journey covers subscribe, refresh, application publish, and unsubscribe.
	fixture := newEventFixture(t)
	result := fixture.subscribe(t, "library.updated", map[string]any{})
	refresh := fixture.subscribe(t, "library.updated", map[string]any{})
	if result["id"] != refresh["id"] || result["cursor"] != nil {
		t.Fatalf("subscription is not stable: %v %v", result, refresh)
	}
	hub := liveevents.New()
	fixture.gateway.ObserveEvents(t.Context(), hub)
	hub.Publish("", "library.updated", "/api/v1/library")
	select {
	case body := <-fixture.received:
		var event struct {
			Name   string            `json:"name"`
			Data   map[string]string `json:"data"`
			Cursor any               `json:"cursor"`
		}
		if json.Unmarshal(body, &event) != nil || event.Name != "library.updated" || event.Data["resource"] != "/api/v1/library" || event.Cursor != nil {
			t.Fatalf("event = %s", body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("event not delivered")
	}
	response := mcpRequest(t, fixture.mux, "events/unsubscribe", map[string]any{"name": "library.updated", "arguments": map[string]any{}, "delivery": map[string]any{"mode": "webhook", "url": "https://receiver.example.test/hooks"}})
	if strings.Contains(response.Body.String(), `"error"`) {
		t.Fatalf("unsubscribe = %s", response.Body.String())
	}
	fixture.gateway.PublishEvent(t.Context(), "", "library.updated", "/api/v1/library")
	select {
	case body := <-fixture.received:
		t.Fatalf("delivery after unsubscribe: %s", body)
	default:
	}
}

func TestEventsRejectInvalidInputWithoutCallbackOrSubscription(t *testing.T) {
	for label, change := range map[string]func(map[string]any){
		"missing name":         func(p map[string]any) { delete(p, "name") },
		"unknown name":         func(p map[string]any) { p["name"] = "unknown" },
		"oversized name":       func(p map[string]any) { p["name"] = strings.Repeat("x", 129) },
		"missing arguments":    func(p map[string]any) { delete(p, "arguments") },
		"unknown arguments":    func(p map[string]any) { p["arguments"] = map[string]any{"profile": "other"} },
		"conflicting resource": func(p map[string]any) { p["arguments"] = map[string]any{"resource": "/api/v1/settings"} },
		"oversized resource":   func(p map[string]any) { p["arguments"] = map[string]any{"resource": strings.Repeat("x", 2049)} },
		"unknown field":        func(p map[string]any) { p["unknown"] = true },
		"unsupported cursor":   func(p map[string]any) { p["cursor"] = "replay" },
		"negative TTL":         func(p map[string]any) { p["ttlMs"] = -1 },
		"fractional TTL":       func(p map[string]any) { p["ttlMs"] = 1.5 },
		"bad mode": func(p map[string]any) {
			p["delivery"] = map[string]any{"mode": "poll", "url": "https://receiver.example.test/hooks", "secret": "invalid"}
		},
		"short secret": func(p map[string]any) {
			p["delivery"].(map[string]any)["secret"] = "whsec_" + base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 23)))
		},
		"long secret": func(p map[string]any) {
			p["delivery"].(map[string]any)["secret"] = "whsec_" + base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 65)))
		},
		"bad secret": func(p map[string]any) { p["delivery"].(map[string]any)["secret"] = "whsec_invalid" },
		"plaintext":  func(p map[string]any) { p["delivery"].(map[string]any)["url"] = "http://receiver.example.test/hooks" },
		"private IP": func(p map[string]any) { p["delivery"].(map[string]any)["url"] = "https://127.0.0.1/hooks" },
		"credentials": func(p map[string]any) {
			p["delivery"].(map[string]any)["url"] = "https://user:password@receiver.example.test/hooks"
		},
		"fragment": func(p map[string]any) {
			p["delivery"].(map[string]any)["url"] = "https://receiver.example.test/hooks#fragment"
		},
	} {
		t.Run(label, func(t *testing.T) {
			fixture := newEventFixture(t)
			fields := map[string]any{"name": "library.updated", "arguments": map[string]any{}, "delivery": map[string]any{"mode": "webhook", "url": "https://receiver.example.test/hooks", "secret": fixture.secret}}
			change(fields)
			response := mcpRequest(t, fixture.mux, "events/subscribe", fields)
			if !strings.Contains(response.Body.String(), `"error"`) {
				t.Fatalf("invalid input accepted: %s", response.Body.String())
			}
			if fixture.verifications.Load() != 0 {
				t.Fatal("rejected input sent a callback")
			}
			fixture.gateway.PublishEvent(context.Background(), "", "library.updated", "/api/v1/library")
			select {
			case body := <-fixture.received:
				t.Fatalf("invalid input caused delivery: %s", body)
			default:
			}
		})
	}
}
