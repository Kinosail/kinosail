package mcpgateway

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEventsCallbackVerificationRejectsRedirectsAndWrongChallenges(t *testing.T) {
	for _, path := range []string{"/bad-challenge", "/redirect"} {
		t.Run(path, func(t *testing.T) {
			fixture := newEventFixture(t)
			response := mcpRequest(t, fixture.mux, "events/subscribe", map[string]any{"name": "library.updated", "arguments": map[string]any{}, "delivery": map[string]any{"mode": "webhook", "url": "https://receiver.example.test" + path, "secret": fixture.secret}})
			if !strings.Contains(response.Body.String(), `"code":-32015`) || strings.Contains(response.Body.String(), "wrong") || strings.Contains(response.Body.String(), "internal") {
				t.Fatalf("callback failure = %s", response.Body.String())
			}
			fixture.connections.mu.Lock()
			count := len(fixture.connections.events)
			fixture.connections.mu.Unlock()
			if count != 0 || fixture.verifications.Load() != 1 {
				t.Fatalf("failed challenge activated subscription or followed redirect: %d %d", count, fixture.verifications.Load())
			}
		})
	}
}

func TestEventsSubtitlesRequireOwnerManagement(t *testing.T) {
	fixture := newEventFixture(t)
	fixture.gateway.subtitleEvents = true
	fields := map[string]any{"name": "subtitles.updated", "arguments": map[string]any{}, "delivery": map[string]any{"mode": "webhook", "url": "https://receiver.example.test/hooks", "secret": fixture.secret}}
	for _, owner := range []bool{false, true} {
		fixture.principals.mu.Lock()
		profile := fixture.principals.values["viewer"]
		profile.Owner = owner
		fixture.principals.values["viewer"] = profile
		fixture.principals.mu.Unlock()
		response := mcpRequest(t, fixture.mux, "events/subscribe", fields)
		if !strings.Contains(response.Body.String(), `"code":-32012`) {
			t.Fatalf("read-only subtitle subscription = %s", response.Body.String())
		}
	}
	fixture.connections.mu.Lock()
	grant := fixture.connections.grants["grant"]
	grant.Scopes = append(grant.Scopes, ManageScope)
	fixture.connections.grants["grant"] = grant
	fixture.connections.mu.Unlock()
	fixture.subscribe(t, "subtitles.updated", map[string]any{})
	fixture.gateway.PublishEvent(t.Context(), "", "subtitles.updated", "/api/v1/subtitle-library")
	expectEvent(t, fixture)
}

func TestEventsSmallAndNoExpiryRequestsGrantFiniteLifetime(t *testing.T) {
	fixture := newEventFixture(t)
	for _, ttl := range []any{nil, 1, 60000} {
		response := mcpRequest(t, fixture.mux, "events/subscribe", map[string]any{"name": "library.updated", "arguments": map[string]any{}, "delivery": map[string]any{"mode": "webhook", "url": "https://receiver.example.test/hooks", "secret": fixture.secret}, "ttlMs": ttl})
		if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `"error"`) || strings.Contains(response.Body.String(), `"refreshBefore":null`) {
			t.Fatalf("TTL %v = %s", ttl, response.Body.String())
		}
		fixture.connections.mu.Lock()
		for _, subscription := range fixture.connections.events {
			remaining := time.Until(time.Unix(subscription.Expires, 0))
			if remaining < 59*time.Second || remaining > 24*time.Hour {
				t.Errorf("TTL %v grants %v", ttl, remaining)
			}
		}
		fixture.connections.mu.Unlock()
	}
	if fixture.verifications.Load() != 1 {
		t.Fatalf("refresh did not reuse verified callback: %d", fixture.verifications.Load())
	}
}

func TestEventsRetrySameBodyThenSuspendUntilRefresh(t *testing.T) { //nolint:cyclop // One callback journey proves retry identity, suspension, truncation, and recovery.
	fixture := newEventFixture(t)
	fixture.subscribe(t, "library.updated", map[string]any{})
	fixture.status.Store(http.StatusServiceUnavailable)
	fixture.gateway.PublishEvent(t.Context(), "", "library.updated", "/api/v1/library")
	var first string
	for attempt := 0; attempt < 3; attempt++ {
		select {
		case body := <-fixture.received:
			if attempt == 0 {
				first = string(body)
			} else if string(body) != first {
				t.Fatalf("retry changed event: %s %s", first, body)
			}
		case <-time.After(7 * time.Second):
			t.Fatal("retry did not arrive")
		}
	}
	// Wait until the last failure has reached the worker, then prove another
	// application update cannot restart retries without an explicit refresh.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		fixture.gateway.eventMu.Lock()
		suspended := false
		for _, delivery := range fixture.gateway.eventRuntime {
			suspended = suspended || delivery.suspended
		}
		fixture.gateway.eventMu.Unlock()
		if suspended {
			break
		}
		time.Sleep(time.Millisecond)
	}
	fixture.gateway.PublishEvent(t.Context(), "", "library.updated", "/api/v1/library")
	expectNoEvent(t, fixture)
	fixture.status.Store(http.StatusNoContent)
	refreshed := fixture.subscribe(t, "library.updated", map[string]any{})
	if refreshed["truncated"] != true {
		t.Fatalf("missing skipped-event signal: %v", refreshed)
	}
	fixture.gateway.PublishEvent(t.Context(), "", "library.updated", "/api/v1/library")
	expectEvent(t, fixture)
}

func TestEventsDoNotRetryGoneOrOversizedDeliveries(t *testing.T) {
	for _, status := range []int{410, 413} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			fixture := newEventFixture(t)
			fixture.subscribe(t, "library.updated", map[string]any{})
			fixture.status.Store(int64(status))
			fixture.gateway.PublishEvent(t.Context(), "", "library.updated", "/api/v1/library")
			expectEvent(t, fixture)
			expectNoEvent(t, fixture)
		})
	}
}

func TestEventsRefreshSurvivesEarlierDeliveryFailure(t *testing.T) {
	fixture := newEventFixture(t)
	fixture.subscribe(t, "library.updated", map[string]any{})
	gate := &eventReplyGate{arrived: make(chan struct{}), resume: make(chan struct{})}
	var resume sync.Once
	t.Cleanup(func() { resume.Do(func() { close(gate.resume) }) })
	fixture.status.Store(http.StatusGone)
	fixture.replyGate.Store(gate)
	fixture.gateway.PublishEvent(t.Context(), "", "library.updated", "/api/v1/library")
	select {
	case <-gate.arrived:
	case <-time.After(3 * time.Second):
		t.Fatal("delivery did not reach the blocked receiver")
	}
	expectEvent(t, fixture)
	fixture.subscribe(t, "library.updated", map[string]any{})
	fixture.status.Store(http.StatusNoContent)
	resume.Do(func() { close(gate.resume) })
	fixture.gateway.PublishEvent(t.Context(), "", "library.updated", "/api/v1/library")
	expectEvent(t, fixture)
}
